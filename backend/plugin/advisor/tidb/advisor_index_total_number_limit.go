package tidb

// Framework code is generated by the generator.

import (
	"context"
	"fmt"
	"slices"

	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/advisor"
	"github.com/bytebase/bytebase/backend/plugin/advisor/catalog"
)

var (
	_ advisor.Advisor = (*IndexTotalNumberLimitAdvisor)(nil)
	_ ast.Visitor     = (*indexTotalNumberLimitChecker)(nil)
)

func init() {
	advisor.Register(storepb.Engine_TIDB, advisor.MySQLIndexTotalNumberLimit, &IndexTotalNumberLimitAdvisor{})
}

// IndexTotalNumberLimitAdvisor is the advisor checking for index total number limit.
type IndexTotalNumberLimitAdvisor struct {
}

// Check checks for index total number limit.
func (*IndexTotalNumberLimitAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*storepb.Advice, error) {
	stmtList, ok := checkCtx.AST.([]ast.StmtNode)
	if !ok {
		return nil, errors.Errorf("failed to convert to StmtNode")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}
	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}
	checker := &indexTotalNumberLimitChecker{
		level:        level,
		title:        string(checkCtx.Rule.Type),
		max:          payload.Number,
		lineForTable: make(map[string]int),
		catalog:      checkCtx.Catalog,
	}

	for _, stmt := range stmtList {
		checker.text = stmt.Text()
		checker.line = stmt.OriginTextPosition()
		(stmt).Accept(checker)
	}

	return checker.generateAdvice(), nil
}

type indexTotalNumberLimitChecker struct {
	adviceList   []*storepb.Advice
	level        storepb.Advice_Status
	title        string
	text         string
	line         int
	max          int
	lineForTable map[string]int
	catalog      *catalog.Finder
}

func (checker *indexTotalNumberLimitChecker) generateAdvice() []*storepb.Advice {
	type tableName struct {
		name string
		line int
	}
	var tableList []tableName

	for k, v := range checker.lineForTable {
		tableList = append(tableList, tableName{
			name: k,
			line: v,
		})
	}
	slices.SortFunc(tableList, func(i, j tableName) int {
		if i.line < j.line {
			return -1
		}
		if i.line > j.line {
			return 1
		}
		return 0
	})

	for _, table := range tableList {
		tableInfo := checker.catalog.Final.FindTable(&catalog.TableFind{TableName: table.name})
		if tableInfo != nil && tableInfo.CountIndex() > checker.max {
			checker.adviceList = append(checker.adviceList, &storepb.Advice{
				Status:        checker.level,
				Code:          advisor.IndexCountExceedsLimit.Int32(),
				Title:         checker.title,
				Content:       fmt.Sprintf("The count of index in table `%s` should be no more than %d, but found %d", table.name, checker.max, tableInfo.CountIndex()),
				StartPosition: common.ConvertANTLRLineToPosition(table.line),
			})
		}
	}

	return checker.adviceList
}

// Enter implements the ast.Visitor interface.
func (checker *indexTotalNumberLimitChecker) Enter(in ast.Node) (ast.Node, bool) {
	switch node := in.(type) {
	case *ast.CreateTableStmt:
		checker.lineForTable[node.Table.Name.O] = node.OriginTextPosition()
	case *ast.AlterTableStmt:
		for _, spec := range node.Specs {
			switch spec.Tp {
			case ast.AlterTableAddColumns:
				for _, column := range spec.NewColumns {
					if createIndex(column) {
						checker.lineForTable[node.Table.Name.O] = node.OriginTextPosition()
						break
					}
				}
			case ast.AlterTableAddConstraint:
				if createIndex(spec.Constraint) {
					checker.lineForTable[node.Table.Name.O] = node.OriginTextPosition()
				}
			case ast.AlterTableChangeColumn, ast.AlterTableModifyColumn:
				if createIndex(spec.NewColumns[0]) {
					checker.lineForTable[node.Table.Name.O] = node.OriginTextPosition()
				}
			default:
			}
		}
	case *ast.CreateIndexStmt:
		checker.lineForTable[node.Table.Name.O] = node.OriginTextPosition()
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*indexTotalNumberLimitChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func createIndex(in ast.Node) bool {
	switch node := in.(type) {
	case *ast.ColumnDef:
		for _, option := range node.Options {
			switch option.Tp {
			case ast.ColumnOptionPrimaryKey, ast.ColumnOptionUniqKey:
				return true
			default:
			}
		}
	case *ast.Constraint:
		switch node.Tp {
		case ast.ConstraintPrimaryKey,
			ast.ConstraintUniq,
			ast.ConstraintUniqKey,
			ast.ConstraintUniqIndex,
			ast.ConstraintKey,
			ast.ConstraintIndex,
			ast.ConstraintFulltext:
			return true
		default:
		}
	}
	return false
}
