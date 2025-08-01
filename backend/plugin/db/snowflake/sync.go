package snowflake

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/db"
	"github.com/bytebase/bytebase/backend/plugin/db/util"
)

var (
	systemSchemas = map[string]bool{
		"information_schema": true,
	}
)

// SyncInstance syncs the instance.
func (d *Driver) SyncInstance(ctx context.Context) (*db.InstanceMetadata, error) {
	version, err := d.getVersion(ctx)
	if err != nil {
		return nil, err
	}

	instanceRoles, err := d.getInstanceRoles(ctx)
	if err != nil {
		return nil, err
	}

	// Query db info
	databases, err := d.getDatabases(ctx)
	if err != nil {
		return nil, err
	}

	var databaseMetadataSlice []*storepb.DatabaseSchemaMetadata
	for _, database := range databases {
		databaseMetadataSlice = append(databaseMetadataSlice, &storepb.DatabaseSchemaMetadata{Name: database})
	}

	return &db.InstanceMetadata{
		Version:   version,
		Databases: databaseMetadataSlice,
		Metadata: &storepb.Instance{
			Roles: instanceRoles,
		},
	}, nil
}

// SyncDBSchema syncs a single database schema.
func (d *Driver) SyncDBSchema(ctx context.Context) (*storepb.DatabaseSchemaMetadata, error) {
	// Query db info
	databases, err := d.getDatabases(ctx)
	if err != nil {
		return nil, err
	}

	databaseMetadata := &storepb.DatabaseSchemaMetadata{
		Name: d.databaseName,
	}
	found := false
	for _, database := range databases {
		if database == d.databaseName {
			found = true
			break
		}
	}
	if !found {
		return nil, common.Errorf(common.NotFound, "database %q not found", d.databaseName)
	}

	schemaList, err := d.getSchemaList(ctx, d.databaseName)
	if err != nil {
		return nil, err
	}
	tableMap, viewMap, err := d.getTableSchema(ctx, d.databaseName)
	if err != nil {
		return nil, err
	}
	streamMap, err := d.getStreamSchema(ctx, d.databaseName)
	if err != nil {
		return nil, err
	}
	taskMap, err := d.getTaskSchema(ctx, d.databaseName)
	if err != nil {
		return nil, err
	}

	for _, schemaName := range schemaList {
		databaseMetadata.Schemas = append(databaseMetadata.Schemas, &storepb.SchemaMetadata{
			Name:    schemaName,
			Tables:  tableMap[schemaName],
			Views:   viewMap[schemaName],
			Streams: streamMap[schemaName],
			Tasks:   taskMap[schemaName],
		})
	}
	return databaseMetadata, nil
}

func (d *Driver) getSchemaList(ctx context.Context, database string) ([]string, error) {
	// Query table info
	var excludedSchemaList []string
	// Skip all system schemas.
	for k := range systemSchemas {
		excludedSchemaList = append(excludedSchemaList, fmt.Sprintf("'%s'", k))
	}
	excludeWhere := fmt.Sprintf("LOWER(SCHEMA_NAME) NOT IN (%s)", strings.Join(excludedSchemaList, ", "))

	query := fmt.Sprintf(`
		SELECT
			SCHEMA_NAME
		FROM "%s".INFORMATION_SCHEMA.SCHEMATA
		WHERE %s ORDER BY SCHEMA_NAME`, database, excludeWhere)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, util.FormatErrorWithQuery(err, query)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var schemaName string
		if err := rows.Scan(&schemaName); err != nil {
			return nil, err
		}
		result = append(result, schemaName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// getStreamSchema returns the stream map of the given database.
//
// Key: normalized schema name
//
// Value: stream list in the schema.
func (d *Driver) getStreamSchema(ctx context.Context, database string) (map[string][]*storepb.StreamMetadata, error) {
	streamMap := make(map[string][]*storepb.StreamMetadata)

	streamQuery := fmt.Sprintf(`SHOW STREAMS IN DATABASE "%s";`, database)
	streamMetaRows, err := d.db.QueryContext(ctx, streamQuery)
	if err != nil {
		return nil, util.FormatErrorWithQuery(err, streamQuery)
	}
	defer streamMetaRows.Close()
	columns, err := streamMetaRows.Columns()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get streams from %q query", streamQuery)
	}
	var streamNameIndex, schemaNameIndex, ownerIndex, commentIndex, tableNameIndex, tpIndex, staleIndex, modeIndex int
	// https://docs.snowflake.com/en/sql-reference/sql/show-streams#output
	for i, n := range columns {
		switch strings.ToLower(n) {
		case "name":
			streamNameIndex = i
		case "schema_name":
			schemaNameIndex = i
		case "owner":
			ownerIndex = i
		case "comment":
			commentIndex = i
		case "table_name":
			tableNameIndex = i
		case "type":
			tpIndex = i
		case "stale":
			staleIndex = i
		case "mode":
			modeIndex = i
		default:
			// Ignore other columns
		}
	}

	for streamMetaRows.Next() {
		cols := make([]any, len(columns))
		var streamName, schemaName, owner, comment, tableName, tp, mode string
		var stale bool
		var unused any
		cols[streamNameIndex] = &streamName
		cols[schemaNameIndex] = &schemaName
		cols[ownerIndex] = &owner
		cols[commentIndex] = &comment
		cols[tableNameIndex] = &tableName
		cols[tpIndex] = &tp
		cols[staleIndex] = &stale
		cols[modeIndex] = &mode
		for i, v := range cols {
			if v == nil {
				cols[i] = &unused
			}
		}

		if err := streamMetaRows.Scan(cols...); err != nil {
			return nil, err
		}
		storePbStreamType := storepb.StreamMetadata_TYPE_UNSPECIFIED
		if tp == "DELTA" {
			storePbStreamType = storepb.StreamMetadata_TYPE_DELTA
		}
		storePbMode := storepb.StreamMetadata_MODE_UNSPECIFIED
		switch mode {
		case "DEFAULT":
			storePbMode = storepb.StreamMetadata_MODE_DEFAULT
		case "APPEND_ONLY":
			storePbMode = storepb.StreamMetadata_MODE_APPEND_ONLY
		case "INSERT_ONLY":
			storePbMode = storepb.StreamMetadata_MODE_INSERT_ONLY
		default:
			// Keep as MODE_UNSPECIFIED for unknown modes
		}
		streamMetadata := &storepb.StreamMetadata{
			Name:      streamName,
			TableName: tableName,
			Owner:     owner,
			Comment:   comment,
			Type:      storePbStreamType,
			Stale:     stale,
			Mode:      storePbMode,
		}
		streamMap[schemaName] = append(streamMap[schemaName], streamMetadata)
	}
	if err := streamMetaRows.Err(); err != nil {
		return nil, err
	}

	for schemaName, streamList := range streamMap {
		for _, stream := range streamList {
			definitionQuery := fmt.Sprintf("SELECT GET_DDL('STREAM', '%s', TRUE);", fmt.Sprintf(`"%s"."%s"."%s"`, database, schemaName, stream.Name))
			var definition string
			if err := d.db.QueryRow(definitionQuery).Scan(&definition); err != nil {
				return nil, err
			}
			stream.Definition = definition
		}
	}

	for _, streamList := range streamMap {
		slices.SortFunc(streamList, func(i, j *storepb.StreamMetadata) int {
			if i.Name < j.Name {
				return -1
			}
			if i.Name > j.Name {
				return 1
			}
			return 0
		})
	}
	return streamMap, nil
}

// ArrayString is a custom type for scanning array of string.
type ArrayString []string

// Scan implements the sql.Scanner interface.
func (a *ArrayString) Scan(src any) error {
	switch v := src.(type) {
	case string:
		return json.Unmarshal([]byte(v), a)
	case []byte:
		return json.Unmarshal(v, a)
	default:
		return errors.New("invalid type")
	}
}

// getTaskSchema returns the task map of the given database.
//
// Key: normalized schema name
//
// Value: stream list in the schema.
func (d *Driver) getTaskSchema(ctx context.Context, database string) (map[string][]*storepb.TaskMetadata, error) {
	taskMap := make(map[string][]*storepb.TaskMetadata)

	taskQuery := fmt.Sprintf(`SHOW TASKS IN DATABASE "%s";`, database)
	streamMetaRows, err := d.db.QueryContext(ctx, taskQuery)
	if err != nil {
		return nil, util.FormatErrorWithQuery(err, taskQuery)
	}
	defer streamMetaRows.Close()
	columns, err := streamMetaRows.Columns()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get tasks from %q query", taskQuery)
	}
	var taskNameIndex, taskIDIndex, schemaNameIndex, ownerIndex, commentIndex, warehouseIndex, nullScheduleIndex, predecessorsIndex, stateIndex, nullConditionIndex int
	// https://docs.snowflake.com/en/sql-reference/sql/show-tasks#output
	for i, n := range columns {
		switch strings.ToLower(n) {
		case "name":
			taskNameIndex = i
		case "id":
			taskIDIndex = i
		case "schema_name":
			schemaNameIndex = i
		case "owner":
			ownerIndex = i
		case "comment":
			commentIndex = i
		case "warehouse":
			warehouseIndex = i
		case "schedule":
			nullScheduleIndex = i
		case "predecessors":
			predecessorsIndex = i
		case "state":
			stateIndex = i
		case "condition":
			nullConditionIndex = i
		default:
			// Ignore other columns
		}
	}
	for streamMetaRows.Next() {
		cols := make([]any, len(columns))
		var taskName, taskID, schemaName, owner, comment, state string
		var nullSchedule, nullCondition, nullWarehouse sql.NullString
		var predecessors ArrayString
		var unused any
		cols[taskNameIndex] = &taskName
		cols[taskIDIndex] = &taskID
		cols[schemaNameIndex] = &schemaName
		cols[ownerIndex] = &owner
		cols[commentIndex] = &comment
		cols[warehouseIndex] = &nullWarehouse
		cols[nullScheduleIndex] = &nullSchedule
		cols[predecessorsIndex] = &predecessors
		cols[stateIndex] = &state
		cols[nullConditionIndex] = &nullCondition
		for i, v := range cols {
			if v == nil {
				cols[i] = &unused
			}
		}
		if err := streamMetaRows.Scan(cols...); err != nil {
			return nil, err
		}
		storePbState := storepb.TaskMetadata_STATE_UNSPECIFIED
		switch state {
		case "started":
			storePbState = storepb.TaskMetadata_STATE_STARTED
		case "suspended":
			storePbState = storepb.TaskMetadata_STATE_SUSPENDED
		default:
			// Keep as STATE_UNSPECIFIED for unknown states
		}
		var schedule, condition, warehouse string
		if nullSchedule.Valid {
			schedule = nullSchedule.String
		}
		if nullCondition.Valid {
			condition = nullCondition.String
		}
		if nullWarehouse.Valid {
			warehouse = nullWarehouse.String
		}
		taskMetadata := &storepb.TaskMetadata{
			Name:         taskName,
			Id:           taskID,
			Owner:        owner,
			Comment:      comment,
			Warehouse:    warehouse,
			Schedule:     schedule,
			Predecessors: predecessors,
			State:        storePbState,
			Condition:    condition,
		}
		taskMap[schemaName] = append(taskMap[schemaName], taskMetadata)
	}
	if err := streamMetaRows.Err(); err != nil {
		return nil, err
	}

	for schemaName, taskList := range taskMap {
		for _, task := range taskList {
			definitionQuery := fmt.Sprintf("SELECT GET_DDL('TASK', '%s', TRUE);", fmt.Sprintf(`"%s"."%s"."%s"`, database, schemaName, task.Name))
			var definition string
			if err := d.db.QueryRow(definitionQuery).Scan(&definition); err != nil {
				return nil, err
			}
			task.Definition = definition
		}
	}

	for _, taskList := range taskMap {
		slices.SortFunc(taskList, func(i, j *storepb.TaskMetadata) int {
			if i.Name < j.Name {
				return -1
			}
			if i.Name > j.Name {
				return 1
			}
			return 0
		})
	}
	return taskMap, nil
}

func (d *Driver) getTableSchema(ctx context.Context, database string) (map[string][]*storepb.TableMetadata, map[string][]*storepb.ViewMetadata, error) {
	tableMap, viewMap := make(map[string][]*storepb.TableMetadata), make(map[string][]*storepb.ViewMetadata)

	// Query table info
	var excludedSchemaList []string
	// Skip all system schemas.
	for k := range systemSchemas {
		excludedSchemaList = append(excludedSchemaList, fmt.Sprintf("'%s'", k))
	}
	excludeWhere := fmt.Sprintf("LOWER(TABLE_SCHEMA) NOT IN (%s)", strings.Join(excludedSchemaList, ", "))

	// Query column info.
	columnMap := make(map[db.TableKey][]*storepb.ColumnMetadata)
	columnQuery := fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			IFNULL(COLUMN_NAME, ''),
			ORDINAL_POSITION,
			COLUMN_DEFAULT,
			IS_NULLABLE,
			DATA_TYPE,
			IFNULL(CHARACTER_SET_NAME, ''),
			IFNULL(COLLATION_NAME, ''),
			IFNULL(COMMENT, '')
		FROM "%s".INFORMATION_SCHEMA.COLUMNS
		WHERE %s
		ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`, database, excludeWhere)
	columnRows, err := d.db.QueryContext(ctx, columnQuery)
	if err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, columnQuery)
	}
	defer columnRows.Close()
	for columnRows.Next() {
		var schemaName, tableName, nullable string
		var defaultStr sql.NullString
		column := &storepb.ColumnMetadata{}
		if err := columnRows.Scan(
			&schemaName,
			&tableName,
			&column.Name,
			&column.Position,
			&defaultStr,
			&nullable,
			&column.Type,
			&column.CharacterSet,
			&column.Collation,
			&column.Comment,
		); err != nil {
			return nil, nil, err
		}
		if defaultStr.Valid {
			// Store in Default field (migration from DefaultExpression to Default)
			column.Default = defaultStr.String
		}
		isNullBool, err := util.ConvertYesNo(nullable)
		if err != nil {
			return nil, nil, err
		}
		column.Nullable = isNullBool

		key := db.TableKey{Schema: schemaName, Table: tableName}
		columnMap[key] = append(columnMap[key], column)
	}
	if err := columnRows.Err(); err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, columnQuery)
	}

	tableQuery := fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			ROW_COUNT,
			BYTES,
			IFNULL(COMMENT, '')
		FROM "%s".INFORMATION_SCHEMA.TABLES
		WHERE TABLE_TYPE = 'BASE TABLE' AND %s
		ORDER BY TABLE_SCHEMA, TABLE_NAME`, database, excludeWhere)
	tableRows, err := d.db.QueryContext(ctx, tableQuery)
	if err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, tableQuery)
	}
	defer tableRows.Close()
	for tableRows.Next() {
		var schemaName string
		table := &storepb.TableMetadata{}
		if err := tableRows.Scan(
			&schemaName,
			&table.Name,
			&table.RowCount,
			&table.DataSize,
			&table.Comment,
		); err != nil {
			return nil, nil, err
		}
		if columns, ok := columnMap[db.TableKey{Schema: schemaName, Table: table.Name}]; ok {
			table.Columns = columns
		}

		tableMap[schemaName] = append(tableMap[schemaName], table)
	}
	if err := tableRows.Err(); err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, tableQuery)
	}

	viewQuery := fmt.Sprintf(`
		SELECT
			TABLE_SCHEMA,
			TABLE_NAME,
			IFNULL(VIEW_DEFINITION, ''),
			IFNULL(COMMENT, '')
		FROM "%s".INFORMATION_SCHEMA.VIEWS
		WHERE %s
		ORDER BY TABLE_SCHEMA, TABLE_NAME`, database, excludeWhere)
	viewRows, err := d.db.QueryContext(ctx, viewQuery)
	if err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, viewQuery)
	}
	defer viewRows.Close()
	for viewRows.Next() {
		view := &storepb.ViewMetadata{}
		var schemaName string
		if err := viewRows.Scan(
			&schemaName,
			&view.Name,
			&view.Definition,
			&view.Comment,
		); err != nil {
			return nil, nil, err
		}
		if columns, ok := columnMap[db.TableKey{Schema: schemaName, Table: view.Name}]; ok {
			for _, column := range columns {
				// TODO(zp): We get column by query the INFORMATION_SCHEMA.COLUMNS, which does not contains the view column belongs to which database.
				// So in the Snowflake, one view column may belongs to different databases, it may cause some confusing behavior in the Data Masking.
				view.DependencyColumns = append(view.DependencyColumns, &storepb.DependencyColumn{
					Schema: schemaName,
					Table:  view.Name,
					Column: column.Name,
				})
			}
		}
		view.Columns = columnMap[db.TableKey{Schema: schemaName, Table: view.Name}]

		viewMap[schemaName] = append(viewMap[schemaName], view)
	}
	if err := viewRows.Err(); err != nil {
		return nil, nil, util.FormatErrorWithQuery(err, viewQuery)
	}

	return tableMap, viewMap, nil
}
