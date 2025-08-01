package tidb

// Framework code is generated by the generator.

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pkg/errors"

	"github.com/bytebase/bytebase/backend/common"
	storepb "github.com/bytebase/bytebase/backend/generated-go/store"
	"github.com/bytebase/bytebase/backend/plugin/advisor"
)

var (
	_ advisor.Advisor = (*CharsetAllowlistAdvisor)(nil)
	_ ast.Visitor     = (*charsetAllowlistChecker)(nil)
)

func init() {
	advisor.Register(storepb.Engine_TIDB, advisor.MySQLCharsetAllowlist, &CharsetAllowlistAdvisor{})
}

// CharsetAllowlistAdvisor is the advisor checking for charset allowlist.
type CharsetAllowlistAdvisor struct {
}

// Check checks for charset allowlist.
func (*CharsetAllowlistAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*storepb.Advice, error) {
	stmtList, ok := checkCtx.AST.([]ast.StmtNode)
	if !ok {
		return nil, errors.Errorf("failed to convert to StmtNode")
	}
	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}
	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}
	checker := &charsetAllowlistChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		allowlist: make(map[string]bool),
	}
	for _, charset := range payload.List {
		checker.allowlist[strings.ToLower(charset)] = true
	}

	for _, stmt := range stmtList {
		checker.text = stmt.Text()
		checker.line = stmt.OriginTextPosition()
		(stmt).Accept(checker)
	}

	return checker.adviceList, nil
}

type charsetAllowlistChecker struct {
	adviceList []*storepb.Advice
	level      storepb.Advice_Status
	title      string
	text       string
	line       int
	allowlist  map[string]bool
}

// Enter implements the ast.Visitor interface.
func (checker *charsetAllowlistChecker) Enter(in ast.Node) (ast.Node, bool) {
	code := advisor.Ok
	var disabledCharset string
	line := checker.line
	switch node := in.(type) {
	case *ast.CreateDatabaseStmt:
		charset := getDatabaseCharset(node.Options)
		if _, exist := checker.allowlist[charset]; charset != "" && !exist {
			code = advisor.DisabledCharset
			disabledCharset = charset
		}
	case *ast.CreateTableStmt:
		charset := getTableCharset(node.Options)
		if _, exist := checker.allowlist[charset]; charset != "" && !exist {
			code = advisor.DisabledCharset
			disabledCharset = charset
			break
		}
		for _, column := range node.Cols {
			charset := getColumnCharset(column)
			if _, exist := checker.allowlist[charset]; charset != "" && !exist {
				code = advisor.DisabledCharset
				disabledCharset = charset
				line = column.OriginTextPosition()
				break
			}
		}
	case *ast.AlterDatabaseStmt:
		charset := getDatabaseCharset(node.Options)
		if _, exist := checker.allowlist[charset]; charset != "" && !exist {
			code = advisor.DisabledCharset
			disabledCharset = charset
		}
	case *ast.AlterTableStmt:
		for _, spec := range node.Specs {
			switch spec.Tp {
			case ast.AlterTableOption:
				charset := getTableCharset(spec.Options)
				if _, exist := checker.allowlist[charset]; charset != "" && !exist {
					code = advisor.DisabledCharset
					disabledCharset = charset
				}
			case ast.AlterTableAddColumns:
				for _, column := range spec.NewColumns {
					charset := getColumnCharset(column)
					if _, exist := checker.allowlist[charset]; charset != "" && !exist {
						code = advisor.DisabledCharset
						disabledCharset = charset
						break
					}
				}
			case ast.AlterTableChangeColumn, ast.AlterTableModifyColumn:
				charset := getColumnCharset(spec.NewColumns[0])
				if _, exist := checker.allowlist[charset]; charset != "" && !exist {
					code = advisor.DisabledCharset
					disabledCharset = charset
				}
			default:
			}
		}
	}

	if code != advisor.Ok {
		checker.adviceList = append(checker.adviceList, &storepb.Advice{
			Status:        checker.level,
			Code:          code.Int32(),
			Title:         checker.title,
			Content:       fmt.Sprintf("\"%s\" used disabled charset '%s'", checker.text, disabledCharset),
			StartPosition: common.ConvertANTLRLineToPosition(line),
		})
	}

	return in, false
}

// Leave implements the ast.Visitor interface.
func (*charsetAllowlistChecker) Leave(in ast.Node) (ast.Node, bool) {
	return in, true
}

func getDatabaseCharset(optionList []*ast.DatabaseOption) string {
	for _, option := range optionList {
		if option.Tp == ast.DatabaseOptionCharset {
			return strings.ToLower(option.Value)
		}
	}

	return ""
}

func getTableCharset(optionList []*ast.TableOption) string {
	for _, option := range optionList {
		if option.Tp == ast.TableOptionCharset {
			if option.Default {
				return ""
			}
			return strings.ToLower(option.StrValue)
		}
	}

	return ""
}

func getColumnCharset(column *ast.ColumnDef) string {
	return strings.ToLower(column.Tp.GetCharset())
}
