package mongodb

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/pkg/errors"

	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/db"
)

var systemCollection = map[string]bool{
	"system.namespaces": true,
	"system.indexes":    true,
	"system.profile":    true,
	"system.js":         true,
	"system.views":      true,
}

var systemDatabase = map[string]bool{
	"admin":    true,
	"config":   true,
	"local":    true,
	"bytebase": true,
}

// UsersInfo is the subset of the mongodb command result of "usersInfo".
type UsersInfo struct {
	Users []User `bson:"users"`
}

// User is the subset of the `users` field in the `User`.
type User struct {
	ID       string `json:"_id" bson:"_id"`
	UserName string `json:"user" bson:"user"`
	DB       string `json:"db" bson:"db"`
	Roles    []Role `json:"roles" bson:"roles"`
}

// Role is the subset of the `roles` field in the `User`.
type Role struct {
	RoleName string `json:"role" bson:"role"`
	DB       string `json:"db" bson:"db"`
}

// SyncInstance syncs the instance meta.
func (d *Driver) SyncInstance(ctx context.Context) (*db.InstanceMetadata, error) {
	version, err := d.getVersion(ctx)
	if err != nil {
		return nil, err
	}
	instanceRoles, err := d.getInstanceRoles(ctx)
	if err != nil {
		return nil, err
	}
	var databases []*storepb.DatabaseSchemaMetadata
	databaseNames, err := d.getNonSystemDatabaseList(ctx)
	if err != nil {
		return nil, err
	}
	for _, databaseName := range databaseNames {
		databases = append(databases, &storepb.DatabaseSchemaMetadata{
			Name: databaseName,
		})
	}

	return &db.InstanceMetadata{
		Version:   version,
		Databases: databases,
		Metadata: &storepb.Instance{
			Roles: instanceRoles,
		},
	}, nil
}

// SyncDBSchema syncs the database schema.
func (d *Driver) SyncDBSchema(ctx context.Context) (*storepb.DatabaseSchemaMetadata, error) {
	schemaMetadata := &storepb.SchemaMetadata{
		Name: "",
	}

	exist, err := d.isDatabaseExist(ctx, d.databaseName)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, errors.Errorf("database %s does not exist", d.databaseName)
	}

	database := d.client.Database(d.databaseName)
	collectionList, err := database.ListCollections(ctx, bson.M{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list collection names")
	}
	var collectionNames []string
	var viewNames []string
	for collectionList.Next(ctx) {
		var collection bson.M
		if err := collectionList.Decode(&collection); err != nil {
			return nil, errors.Wrap(err, "failed to decode collection")
		}
		var tp string
		if t, ok := collection["type"]; ok {
			if s, ok := t.(string); ok && s == "collection" {
				tp = "collection"
			}
			if s, ok := t.(string); ok && s == "view" {
				tp = "view"
			}
		}
		name, ok := collection["name"]
		if !ok {
			return nil, errors.New("cannot get collection name from collection info")
		}
		collectionName, ok := name.(string)
		if !ok {
			return nil, errors.New("cannot convert collection name to string")
		}
		switch tp {
		case "collection":
			collectionNames = append(collectionNames, collectionName)
		case "view":
			viewNames = append(viewNames, collectionName)
		default:
			// Other types like system collections
		}
	}
	if err := collectionList.Err(); err != nil {
		return nil, errors.Wrap(err, "failed to list collection names")
	}
	if err := collectionList.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close collection list")
	}
	slices.Sort(collectionNames)
	slices.Sort(viewNames)

	for _, collectionName := range collectionNames {
		if systemCollection[collectionName] {
			continue
		}

		collection := database.Collection(collectionName)
		count, err := collection.EstimatedDocumentCount(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get estimated document count")
		}
		// Get collection data size and total index size in byte.
		var commandResult bson.M
		if err := database.RunCommand(ctx, bson.D{{
			Key:   "collStats",
			Value: collectionName,
		}}).Decode(&commandResult); err != nil {
			return nil, errors.Wrap(err, "cannot run collStats command")
		}
		dataSize, ok := commandResult["size"]
		if !ok {
			return nil, errors.New("cannot get size from collStats command result")
		}
		dataSize64, err := convertEmptyInterfaceToInt64(dataSize)
		if err != nil {
			slog.Debug("Failed to convert dataSize to int64", slog.Any("dataSize", dataSize))
		}

		totalIndexSize, ok := commandResult["totalIndexSize"]
		if !ok {
			return nil, errors.New("cannot get totalIndexSize from collStats command result")
		}
		totalIndexSize64, err := convertEmptyInterfaceToInt64(totalIndexSize)
		if err != nil {
			slog.Debug("Failed to convert totalIndexSize to int64", slog.Any("totalIndexSize", totalIndexSize))
		}

		// Get collection indexes.
		indexes, err := getIndexes(ctx, collection)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get index schema of collection %s", collectionName)
		}
		schemaMetadata.Tables = append(schemaMetadata.Tables, &storepb.TableMetadata{
			Name:      collectionName,
			RowCount:  count,
			DataSize:  dataSize64,
			IndexSize: totalIndexSize64,
			Indexes:   indexes,
		})
	}

	for _, viewName := range viewNames {
		schemaMetadata.Views = append(schemaMetadata.Views, &storepb.ViewMetadata{Name: viewName})
	}

	return &storepb.DatabaseSchemaMetadata{
		Name:    d.databaseName,
		Schemas: []*storepb.SchemaMetadata{schemaMetadata},
	}, nil
}

// getIndexes returns all indexes schema of a collection.
// https://www.mongodb.com/docs/manual/reference/command/listIndexes/#output
func getIndexes(ctx context.Context, collection *mongo.Collection) ([]*storepb.IndexMetadata, error) {
	indexCursor, err := collection.Indexes().List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list indexes")
	}
	indexMap := make(map[string]*storepb.IndexMetadata)
	defer indexCursor.Close(ctx)
	for indexCursor.Next(ctx) {
		var indexInfo bson.M
		if err := indexCursor.Decode(&indexInfo); err != nil {
			return nil, errors.Wrap(err, "failed to decode index info")
		}
		name, ok := indexInfo["name"]
		if !ok {
			return nil, errors.New("cannot get index name from index info")
		}
		indexName, ok := name.(string)
		if !ok {
			return nil, errors.New("cannot cinvert index name to string")
		}
		key, ok := indexInfo["key"]
		if !ok {
			return nil, errors.New("cannot get index key from index info")
		}
		expression, err := json.Marshal(key)
		if err != nil {
			return nil, errors.Wrap(err, "cannot marshal index key to json")
		}
		unique := false
		if u, ok := indexInfo["unique"]; ok {
			unique, ok = u.(bool)
			if !ok {
				return nil, errors.New("cannot convert unique to bool")
			}
		}

		if _, ok := indexMap[indexName]; !ok {
			indexMap[indexName] = &storepb.IndexMetadata{
				Name:   indexName,
				Unique: unique,
			}
		}
		indexMap[indexName].Expressions = append(indexMap[indexName].Expressions, string(expression))
	}

	var indexes []*storepb.IndexMetadata
	var indexNames []string
	for name := range indexMap {
		indexNames = append(indexNames, name)
	}
	slices.Sort(indexNames)
	for _, name := range indexNames {
		indexes = append(indexes, indexMap[name])
	}
	return indexes, nil
}

// getVersion returns the version of mongod or mongos instance.
func (d *Driver) getVersion(ctx context.Context) (string, error) {
	database := d.client.Database(bytebaseDefaultDatabase)
	var commandResult bson.M
	command := bson.D{{Key: "buildInfo", Value: 1}}
	if err := database.RunCommand(ctx, command).Decode(&commandResult); err != nil {
		return "", errors.Wrap(err, "cannot run buildInfo command")
	}
	version, ok := commandResult["version"]
	if !ok {
		return "", errors.New("cannot get version from buildInfo command result")
	}
	v, ok := version.(string)
	if !ok {
		return "", errors.New("cannot convert version to string")
	}
	return v, nil
}

// isDatabaseExist returns true if the database exists.
func (d *Driver) isDatabaseExist(ctx context.Context, databaseName string) (bool, error) {
	// We do the filter by hand instead of using the filter option of ListDatabaseNames because we may encounter the following error:
	// Unallowed argument in listDatabases command: filter
	databaseList, err := d.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return false, errors.Wrap(err, "failed to list database names")
	}
	for _, database := range databaseList {
		if database == databaseName {
			return true, nil
		}
	}
	return false, nil
}

// getNonSystemDatabaseList returns the list of non system databases.
func (d *Driver) getNonSystemDatabaseList(ctx context.Context) ([]string, error) {
	databaseNames, err := d.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list database names")
	}
	var nonSystemDatabaseList []string
	for _, databaseName := range databaseNames {
		if _, ok := systemDatabase[databaseName]; !ok {
			nonSystemDatabaseList = append(nonSystemDatabaseList, databaseName)
		}
	}
	return nonSystemDatabaseList, nil
}

// isAtlasUnauthorizedError returns true if the error is an Atlas unauthorized error.
func isAtlasUnauthorizedError(err error) bool {
	commandError, ok := err.(mongo.CommandError)
	if !ok {
		return strings.Contains(err.Error(), "AtlasError: Unauthorized")
	}
	// Atlas M0/M2/M5 shared cluster does not support usersInfo command.
	if commandError.Name == "AtlasError" && commandError.Code == 8000 && strings.Contains(commandError.Message, "Unauthorized") {
		return true
	}
	// M10/M20/M30 returns the following error.
	if commandError.Name == "Unauthorized" && commandError.Code == 13 {
		return true
	}

	return false
}

func convertEmptyInterfaceToInt64(value any) (int64, error) {
	// NOTE: convert uint64 to int64 may cause overflow. But we don't care about it.
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	default:
		return 0, errors.Errorf("cannot convert %v to int64", value)
	}
}
