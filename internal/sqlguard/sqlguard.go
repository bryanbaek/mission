// Package sqlguard is the AST-level safety layer that sits between any
// LLM-emitted SQL and the MySQL gateway. It rejects anything that is not a
// read-only SELECT/WITH/SHOW statement and auto-injects a LIMIT cap on
// top-level queries whose result set is not already bounded by LIMIT or by
// top-level aggregation.
package sqlguard

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver" // registers the value-expr driver
)

// DefaultRowLimit is the cap auto-injected into un-aggregated top-level
// SELECT statements that do not already declare a LIMIT.
const DefaultRowLimit = 1000

type Result struct {
	OK                bool
	RewrittenSQL      string
	Reason            string
	BlockedConstructs []string
	LimitInjected     bool
}

func Validate(sqlText string) (Result, error) {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return deny(
			"SQL could not be parsed as a single MySQL statement",
			"parse_error",
		), nil
	}

	if hasCommentToken(trimmed) {
		return deny("SQL comments are not allowed", "comment"), nil
	}
	if result, ok := detectRawDeniedPattern(trimmed); ok {
		return result, nil
	}

	p := parser.New()
	stmts, _, err := p.Parse(trimmed, "", "")
	if err != nil {
		return deny(
			"SQL could not be parsed as a single MySQL statement",
			"parse_error",
		), nil
	}
	if len(stmts) == 0 {
		return deny(
			"SQL could not be parsed as a single MySQL statement",
			"parse_error",
		), nil
	}
	if len(stmts) > 1 {
		return deny(
			"multi-statement SQL is not allowed",
			"multi_statement",
		), nil
	}

	stmt := stmts[0]
	switch stmt.(type) {
	case *ast.SelectStmt, *ast.SetOprStmt, *ast.ShowStmt:
		// Allowed top-level statement kinds.
	default:
		return deny(
			"only SELECT, WITH-that-resolves-to-SELECT, and SHOW statements are allowed",
			statementConstruct(stmt),
		), nil
	}

	v := &guardVisitor{}
	stmt.Accept(v)
	if v.denied != nil {
		return *v.denied, nil
	}

	needsLimit, err := needsLimitInjection(stmt)
	if err != nil {
		return Result{}, err
	}

	var buf strings.Builder
	restoreCtx := format.NewRestoreCtx(
		format.DefaultRestoreFlags|format.RestoreStringWithoutCharset,
		&buf,
	)
	if err := stmt.Restore(restoreCtx); err != nil {
		return Result{}, fmt.Errorf("restore sql: %w", err)
	}

	rewritten := buf.String()
	if needsLimit {
		rewritten = fmt.Sprintf("%s LIMIT %d", rewritten, DefaultRowLimit)
	}

	return Result{
		OK:            true,
		RewrittenSQL:  rewritten,
		LimitInjected: needsLimit,
	}, nil
}

type guardVisitor struct {
	denied *Result
}

func (v *guardVisitor) Enter(n ast.Node) (ast.Node, bool) {
	if v.denied != nil {
		return n, true
	}

	switch node := n.(type) {
	case *ast.SelectStmt:
		if node.SelectIntoOpt != nil {
			v.denied = resultPtr(
				deny(
					"SELECT ... INTO is not allowed",
					"select_into",
				),
			)
			return n, true
		}
		if node.LockInfo != nil {
			switch node.LockInfo.LockType {
			case ast.SelectLockNone,
				ast.SelectLockForShare,
				ast.SelectLockForShareNoWait,
				ast.SelectLockForShareSkipLocked:
				// Read-oriented share locks are allowed.
			default:
				v.denied = resultPtr(
					deny(
						"SELECT ... FOR UPDATE is not allowed",
						"for_update",
					),
				)
				return n, true
			}
		}
	}
	return n, false
}

func (v *guardVisitor) Leave(n ast.Node) (ast.Node, bool) {
	return n, v.denied == nil
}

func deny(reason string, blockedConstructs ...string) Result {
	return Result{
		OK:                false,
		Reason:            reason,
		BlockedConstructs: uniqueConstructs(blockedConstructs),
	}
}

func resultPtr(result Result) *Result {
	return &result
}

func uniqueConstructs(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func needsLimitInjection(stmt ast.StmtNode) (bool, error) {
	switch s := stmt.(type) {
	case *ast.SelectStmt:
		if s.Limit != nil {
			return false, nil
		}
		if isAggregatedSelect(s) {
			return false, nil
		}
		return true, nil
	case *ast.SetOprStmt:
		if s.Limit != nil {
			return false, nil
		}
		return true, nil
	case *ast.ShowStmt:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected statement type %T", stmt)
	}
}

func isAggregatedSelect(s *ast.SelectStmt) bool {
	if s.GroupBy != nil && len(s.GroupBy.Items) > 0 {
		return true
	}
	if s.Fields == nil || len(s.Fields.Fields) == 0 {
		return false
	}
	for _, field := range s.Fields.Fields {
		if field == nil || field.Expr == nil {
			continue
		}
		if containsTopLevelAggregate(field.Expr) {
			return true
		}
	}
	return false
}

func containsTopLevelAggregate(expr ast.ExprNode) bool {
	switch e := expr.(type) {
	case *ast.AggregateFuncExpr:
		return true
	case *ast.ParenthesesExpr:
		return containsTopLevelAggregate(e.Expr)
	default:
		return false
	}
}

func hasCommentToken(sqlText string) bool {
	return scanOutsideQuotes(sqlText, func(fragment string) bool {
		return strings.Contains(fragment, "#") ||
			strings.Contains(fragment, "/*") ||
			strings.Contains(fragment, "-- ")
	}, false)
}

func detectRawDeniedPattern(sqlText string) (Result, bool) {
	normalized := outsideQuotesUpper(sqlText)
	switch {
	case strings.Contains(normalized, " INTO OUTFILE"),
		strings.Contains(normalized, " INTO DUMPFILE"),
		strings.Contains(normalized, " INTO @"):
		return deny("SELECT ... INTO is not allowed", "select_into"), true
	default:
		return Result{}, false
	}
}

func outsideQuotesUpper(sqlText string) string {
	var builder strings.Builder
	builder.Grow(len(sqlText))
	scanOutsideQuotes(sqlText, func(fragment string) bool {
		builder.WriteString(strings.ToUpper(fragment))
		return false
	}, true)
	return builder.String()
}

func scanOutsideQuotes(
	sqlText string,
	outsideFn func(fragment string) bool,
	keepQuotedWidth bool,
) bool {
	const (
		stateNormal = iota
		stateSingle
		stateDouble
		stateBacktick
	)

	state := stateNormal
	start := 0
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]

		switch state {
		case stateNormal:
			switch ch {
			case '\'':
				if outsideFn(sqlText[start:i]) {
					return true
				}
				state = stateSingle
				start = i
			case '"':
				if outsideFn(sqlText[start:i]) {
					return true
				}
				state = stateDouble
				start = i
			case '`':
				if outsideFn(sqlText[start:i]) {
					return true
				}
				state = stateBacktick
				start = i
			case '#':
				return true
			case '/':
				if i+1 < len(sqlText) && sqlText[i+1] == '*' {
					return true
				}
			case '-':
				if i+2 < len(sqlText) &&
					sqlText[i+1] == '-' &&
					isSpaceOrControl(sqlText[i+2]) {
					return true
				}
			}
		case stateSingle:
			if ch == '\\' && i+1 < len(sqlText) {
				i++
				continue
			}
			if ch == '\'' {
				if i+1 < len(sqlText) && sqlText[i+1] == '\'' {
					i++
					continue
				}
				state = stateNormal
				if keepQuotedWidth {
					start = i + 1
				} else {
					start = i + 1
				}
			}
		case stateDouble:
			if ch == '\\' && i+1 < len(sqlText) {
				i++
				continue
			}
			if ch == '"' {
				if i+1 < len(sqlText) && sqlText[i+1] == '"' {
					i++
					continue
				}
				state = stateNormal
				start = i + 1
			}
		case stateBacktick:
			if ch == '`' {
				if i+1 < len(sqlText) && sqlText[i+1] == '`' {
					i++
					continue
				}
				state = stateNormal
				start = i + 1
			}
		}
	}

	if state == stateNormal {
		return outsideFn(sqlText[start:])
	}
	return false
}

func isSpaceOrControl(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

func statementConstruct(stmt ast.StmtNode) string {
	switch node := stmt.(type) {
	case *ast.AlterTableStmt:
		return "statement_alter"
	case *ast.BeginStmt:
		return "statement_begin"
	case *ast.CallStmt:
		return "statement_call"
	case *ast.CommitStmt:
		return "statement_commit"
	case *ast.CreateDatabaseStmt, *ast.CreateTableStmt, *ast.CreateViewStmt, *ast.CreateIndexStmt:
		return "statement_create"
	case *ast.DeleteStmt:
		return "statement_delete"
	case *ast.DoStmt:
		return "statement_do"
	case *ast.DropDatabaseStmt, *ast.DropTableStmt:
		return "statement_drop"
	case *ast.FlushStmt:
		return "statement_flush"
	case *ast.GrantStmt:
		return "statement_grant"
	case *ast.InsertStmt:
		if node.IsReplace {
			return "statement_replace"
		}
		return "statement_insert"
	case *ast.KillStmt:
		return "statement_kill"
	case *ast.LoadDataStmt:
		return "statement_load_data"
	case *ast.LockTablesStmt:
		return "statement_lock_tables"
	case *ast.RenameTableStmt:
		return "statement_rename_table"
	case *ast.RevokeStmt:
		return "statement_revoke"
	case *ast.RollbackStmt:
		return "statement_rollback"
	case *ast.SetStmt:
		return "statement_set"
	case *ast.TruncateTableStmt:
		return "statement_truncate"
	case *ast.UnlockTablesStmt:
		return "statement_unlock_tables"
	case *ast.UpdateStmt:
		return "statement_update"
	default:
		return "statement_disallowed"
	}
}
