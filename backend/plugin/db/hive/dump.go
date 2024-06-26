package hive

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
)

const (
	schemaStmtFmt = "" +
		"--\n" +
		"-- %s structure for %s\n" +
		"--\n" +
		"%s;\n"
)

type IndexDDLOptions struct {
	databaseName          string
	indexName             string
	tableName             string
	indexType             string
	colNames              []string
	isWithDeferredRebuild bool
	idxProperties         map[string]string
	indexTableName        string
	rowFmt                string
	storedAs              string
	storedBy              string
	location              string
	tableProperties       map[string]string
	comment               string
}

type MaterializedViewDDLOptions struct {
	databaseName   string
	mtViewName     string
	disableRewrite bool
	comment        string
	partitionedOn  []string
	clusteredOn    []string
	distributedOn  []string
	sortedOn       []string
	rowFmt         string
	storedAs       string
	storedBy       string
	serdProperties map[string]string
	location       string
	tblProperties  map[string]string
	as             string
}

func (d *Driver) Dump(ctx context.Context, out io.Writer) (string, error) {
	instanceMetadata, err := d.SyncInstance(ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to sync instance")
	}
	var builder strings.Builder

	for _, database := range instanceMetadata.Databases {
		schema := database.Schemas[0]

		// dump databases.
		databaseDDL, err := d.showCreateSchemaDDL(ctx, "DATABASE", schema.Name, "")
		if err != nil {
			return "", errors.Wrapf(err, "failed to dump database %s", schema.Name)
		}
		_, _ = builder.WriteString(databaseDDL)

		// dump managed tables.
		for _, table := range schema.Tables {
			tabDDL, err := d.showCreateSchemaDDL(ctx, "TABLE", table.Name, schema.Name)
			if err != nil {
				return "", errors.Wrapf(err, "failed to dump table %s", table.Name)
			}

			// dump indexes.
			for _, index := range table.Indexes {
				indexDDL, err := genIndexDDL(&IndexDDLOptions{
					databaseName:          schema.Name,
					indexName:             index.Name,
					tableName:             table.Name,
					indexType:             index.Type,
					colNames:              index.Expressions,
					isWithDeferredRebuild: true,
					idxProperties:         nil,
					indexTableName:        "",
					rowFmt:                "",
					storedAs:              "",
					storedBy:              "",
					location:              "",
					tableProperties:       nil,
					comment:               index.Comment,
				})
				if err != nil {
					return "", errors.Wrapf(err, "failed to generate DDL for index %s", index.Name)
				}

				_, _ = builder.WriteString(fmt.Sprintf(schemaStmtFmt, "INDEX", fmt.Sprintf("`%s`", index.Name), indexDDL))
			}
			_, _ = builder.WriteString(tabDDL)
		}

		// dump external tables.
		for _, extTable := range schema.ExternalTables {
			tabDDL, err := d.showCreateSchemaDDL(ctx, "TABLE", extTable.Name, schema.Name)
			if err != nil {
				return "", errors.Wrapf(err, "failed to dump table %s", extTable.Name)
			}
			_, _ = builder.WriteString(tabDDL)
		}

		// dump views.
		for _, view := range schema.Views {
			viewDDL, err := d.showCreateSchemaDDL(ctx, "VIEW", view.Name, schema.Name)
			if err != nil {
				return "", errors.Wrapf(err, "failed to dump view %s", view.Name)
			}
			_, _ = builder.WriteString(viewDDL)
		}

		// dump materialized views.
		for _, mtView := range schema.MaterializedViews {
			mtViewDDL, err := genMaterializedViewDDL(&MaterializedViewDDLOptions{
				databaseName:   schema.Name,
				mtViewName:     mtView.Name,
				disableRewrite: false,
				comment:        mtView.Comment,
				partitionedOn:  nil,
				clusteredOn:    nil,
				distributedOn:  nil,
				sortedOn:       nil,
				rowFmt:         "",
				storedAs:       "",
				storedBy:       "",
				serdProperties: nil,
				location:       "",
				tblProperties:  nil,
				as:             mtView.Definition,
			})
			if err != nil {
				return "", errors.Wrapf(err, "failed to generate DDL for materialized view %s", mtView.Name)
			}

			_, _ = builder.WriteString(fmt.Sprintf(schemaStmtFmt, "MATERIALIZED VIEW", fmt.Sprintf("`%s`.`%s`", schema.Name, mtView.Name), mtViewDDL))
		}
	}

	_, _ = io.WriteString(out, builder.String())
	return builder.String(), nil
}

// This function shows DDLs for creating certain type of schema [VIEW, DATABASE, TABLE].
func (d *Driver) showCreateSchemaDDL(ctx context.Context, schemaType string, schemaName string, belongTo string) (string, error) {
	schemaName = fmt.Sprintf("`%s`", schemaName)
	originalTableName := schemaName
	// rename table to format: [DatabaseName].[TableName].
	if belongTo != "" {
		belongTo = fmt.Sprintf("`%s`", belongTo)
		schemaName = fmt.Sprintf("%s.%s", belongTo, schemaName)
	}

	// 'SHOW CREATE TABLE' can also be used for dumping views.
	queryStatement := fmt.Sprintf("SHOW CREATE %s %s", schemaType, schemaName)
	if schemaType == "VIEW" {
		queryStatement = fmt.Sprintf("SHOW CREATE TABLE %s", schemaName)
	}

	schemaDDLResults, err := d.QueryWithConn(ctx, nil, queryStatement, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to dump %s %s", schemaType, schemaName)
	}

	var schemaDDL string
	for _, row := range schemaDDLResults[0].Rows {
		schemaDDL += fmt.Sprintln(row.Values[0].GetStringValue())
	}

	if belongTo != "" {
		schemaDDL = strings.Replace(schemaDDL, originalTableName, schemaName, 1)
	}

	return fmt.Sprintf(schemaStmtFmt, schemaType, schemaName, schemaDDL), nil
}

func genIndexDDL(opts *IndexDDLOptions) (string, error) {
	var builder strings.Builder

	// index name, table name.
	_, _ = builder.WriteString(fmt.Sprintf("CREATE INDEX `%s`\nON TABLE %s ", opts.indexName, fmt.Sprintf("`%s`.`%s`", opts.databaseName, opts.tableName)))

	// column names.
	formatColumn(&builder, opts.colNames)

	// index type.
	_, _ = builder.WriteString(fmt.Sprintf(")\nAS '%s'\n", opts.indexType))

	// with deferred rebuild.
	if opts.isWithDeferredRebuild {
		_, _ = builder.WriteString("WITH DEFERRED REBUILD\n")
	}

	// index properties.
	if opts.idxProperties != nil {
		_, _ = builder.WriteString("IDXPROPERTIES ")
		formatKVPair(&builder, opts.tableProperties)
	}

	// index table.
	if opts.indexTableName != "" {
		_, _ = builder.WriteString(fmt.Sprintf("IN TABLE %s\n", opts.indexTableName))
	}

	// row format.
	if opts.rowFmt != "" {
		_, _ = builder.WriteString(opts.rowFmt)
		_, _ = builder.WriteRune('\n')
	}

	// stored as or stored by.
	if opts.storedAs != "" && opts.storedBy != "" {
		return "", errors.New("keywords 'STORED AS' and 'STORED BY' cannot appear at the same time")
	} else if opts.storedAs != "" {
		_, _ = builder.WriteString(fmt.Sprintf("STORED AS %s\n", opts.storedAs))
	} else if opts.storedBy != "" {
		_, _ = builder.WriteString(fmt.Sprintf("STORED BY '%s'\n", opts.storedBy))
	}

	// location.
	if opts.location != "" {
		_, _ = builder.WriteString(opts.location)
		_, _ = builder.WriteRune('\n')
	}

	// table properties.
	if opts.tableProperties != nil {
		_, _ = builder.WriteString("TBLPROPERTIES (")
		formatKVPair(&builder, opts.tableProperties)
	}

	// comment.
	if opts.comment != "" {
		_, _ = builder.WriteString(fmt.Sprintf("COMMENT '%s'\n", opts.comment))
	}

	return builder.String(), nil
}

func genMaterializedViewDDL(opts *MaterializedViewDDLOptions) (string, error) {
	var builder strings.Builder
	viewNameWithDB := fmt.Sprintf("`%s`.`%s`", opts.databaseName, opts.mtViewName)

	// mtView name.
	_, _ = builder.WriteString(fmt.Sprintf("CREATE MATERIALIZED VIEW %s\n", viewNameWithDB))

	// disable rewrite.
	if opts.disableRewrite {
		_, _ = builder.WriteString("DISABLE REWRITE\n")
	}

	// comment.
	if opts.comment != "" {
		_, _ = builder.WriteString(fmt.Sprintf("COMMENT '%s'\n", opts.comment))
	}

	// partitioned on.
	if opts.partitionedOn != nil {
		_, _ = builder.WriteString("PARTITIONED ON ")
		formatColumn(&builder, opts.partitionedOn)
	}

	if opts.clusteredOn != nil && opts.distributedOn != nil {
		return "", errors.New("keywords 'CLUSTERED ON' and 'DISTRIBUTED ON' cannot appear at the same time")
	} else if opts.clusteredOn != nil {
		// clusteredOn
		_, _ = builder.WriteString("CLUSTERED ON ")
		formatColumn(&builder, opts.clusteredOn)
	} else if opts.distributedOn != nil && opts.sortedOn != nil {
		// distributed on.
		_, _ = builder.WriteString("DISTRIBUTED ON ")
		formatColumn(&builder, opts.distributedOn)

		// sorted on.
		_, _ = builder.WriteString("SORTED ON ")
		formatColumn(&builder, opts.sortedOn)
	}

	// stored as or stored by.
	if opts.storedAs != "" && opts.storedBy != "" {
		return "", errors.New("keywords 'STORED AS' and 'STORED BY' cannot appear at the same time")
	} else if opts.storedAs != "" {
		_, _ = builder.WriteString(fmt.Sprintf("STORED AS %s\n", opts.storedAs))
	} else if opts.storedBy != "" {
		_, _ = builder.WriteString(fmt.Sprintf("STORED BY '%s'\n", opts.storedBy))

		// if with serde properties.
		if opts.serdProperties != nil {
			_, _ = builder.WriteString("WITH SERDEPROPERTIES ")
			formatKVPair(&builder, opts.serdProperties)
		}
	}

	// location.
	if opts.location != "" {
		_, _ = builder.WriteString(opts.location)
		_, _ = builder.WriteRune('\n')
	}

	// table properties.
	if opts.tblProperties != nil {
		_, _ = builder.WriteString("TBLPROPERTIES ")
		formatKVPair(&builder, opts.tblProperties)
	}

	// as.
	_, _ = builder.WriteString(fmt.Sprintf("AS %s \n", opts.as))

	return builder.String(), nil
}

func formatColumn(builder *strings.Builder, columnNames []string) {
	_, _ = builder.WriteString("(\n")
	for idx, colName := range columnNames {
		_, _ = builder.WriteString(fmt.Sprintf("  `%s`", colName))
		if idx != len(columnNames)-1 {
			_, _ = builder.WriteString(",\n")
		}
	}
	_, _ = builder.WriteString(")\n")
}

func formatKVPair(builder *strings.Builder, kvMap map[string]string) {
	_, _ = builder.WriteString("(\n")
	index := 0
	for key, value := range kvMap {
		_, _ = builder.WriteString(fmt.Sprintf("  '%s' = '%s'", key, value))
		if index != len(kvMap)-1 {
			_, _ = builder.WriteString(",\n")
		}
		index++
	}
	_, _ = builder.WriteString(")\n")
}
