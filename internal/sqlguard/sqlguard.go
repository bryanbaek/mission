// Package sqlguard is the AST-level safety layer that sits between any
// LLM-emitted SQL and the MySQL gateway. It rejects anything that is not a
// read-only SELECT/WITH statement and auto-injects a LIMIT cap on queries
// whose result set is not already bounded by LIMIT or by top-level
// aggregation.
//
// The read-only MySQL user on the edge agent is the primary belt. sqlguard
// is the suspenders — the layer that catches creative injection attempts
// (comment smuggling, multi-statement, INTO OUTFILE, stored procedure calls)
// before they ever reach the database.
package sqlguard

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/format"
	_ "github.com/pingcap/tidb/pkg/parser/test_driver" // registers the value-expr driver
)

// DefaultRowLimit is the cap auto-injected into un-aggregated top-level
// SELECT statements that do not already declare a LIMIT. 1000 is large
// enough to answer realistic SMB questions and small enough that an
// accidentally unbounded scan cannot overwhelm a chat UI.
const DefaultRowLimit = 1000

// Result describes what sqlguard produced for a given input.
//
// RewrittenSQL is always safe to send to MySQL (given the read-only user).
// It is the canonical restored form of the input, optionally with a LIMIT
// appended. Comments and hints are stripped by restoration, which is
// intentional — the restored SQL is what ran, and that is what the audit
// log should record.
type Result struct {
	RewrittenSQL  string
	LimitInjected bool
}

// Validate parses sqlText, enforces the allowlist, and returns the
// rewritten SQL along with a LimitInjected flag.
//
// The allowlist is narrow on purpose: exactly one top-level statement,
// which must be SELECT or a UNION/INTERSECT/EXCEPT of SELECTs (both of
// which may carry a WITH prefix). Anything else — DDL, DML, CALL,
// SELECT ... INTO OUTFILE, multi-statement, unparseable input — is
// rejected.
func Validate(sqlText string) (Result, error) {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return Result{}, errors.New("sql is empty")
	}

	p := parser.New()
	stmts, _, err := p.Parse(trimmed, "", "")
	if err != nil {
		return Result{}, fmt.Errorf("parse sql: %w", err)
	}
	if len(stmts) == 0 {
		return Result{}, errors.New("no statement found")
	}
	if len(stmts) > 1 {
		return Result{}, fmt.Errorf(
			"multi-statement sql is not allowed (got %d statements)",
			len(stmts),
		)
	}

	stmt := stmts[0]
	switch stmt.(type) {
	case *ast.SelectStmt, *ast.SetOprStmt:
		// allowed
	default:
		return Result{}, fmt.Errorf(
			"only SELECT statements are allowed, got %T",
			stmt,
		)
	}

	v := &guardVisitor{}
	stmt.Accept(v)
	if v.err != nil {
		return Result{}, v.err
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
		RewrittenSQL:  rewritten,
		LimitInjected: needsLimit,
	}, nil
}

// guardVisitor walks the AST and rejects nodes that the allowlist treats as
// unsafe. A visitor is used instead of a hand-rolled switch so that every
// SELECT nested in a subquery, UNION leg, or CTE is checked — not only the
// outermost statement.
type guardVisitor struct {
	err error
}

func (v *guardVisitor) Enter(n ast.Node) (ast.Node, bool) {
	if v.err != nil {
		return n, true
	}

	switch node := n.(type) {
	case *ast.SelectStmt:
		if node.SelectIntoOpt != nil {
			v.err = errors.New(
				"SELECT ... INTO OUTFILE/DUMPFILE/@var is not allowed",
			)
			return n, true
		}
		if node.LockInfo != nil {
			switch node.LockInfo.LockType {
			case ast.SelectLockNone,
				ast.SelectLockForShare,
				ast.SelectLockForShareNoWait,
				ast.SelectLockForShareSkipLocked:
				// read locks are acceptable under a read-only user
			default:
				v.err = errors.New(
					"SELECT ... FOR UPDATE is not allowed",
				)
				return n, true
			}
		}
	}
	return n, false
}

func (v *guardVisitor) Leave(n ast.Node) (ast.Node, bool) {
	return n, v.err == nil
}

// needsLimitInjection reports whether the outermost statement should have
// a LIMIT clause appended. The rule:
//
//   - If a top-level LIMIT already exists, respect it.
//   - UNION/INTERSECT/EXCEPT sets without a LIMIT always get one injected.
//   - A plain SELECT without LIMIT gets one injected unless the result is
//     already bounded by top-level aggregation (GROUP BY, or a pure-
//     aggregate projection with no grouping — e.g. SELECT COUNT(*)).
//
// We deliberately do not inject LIMIT into aggregated queries because a
// LIMIT on e.g. `SELECT COUNT(*)` is harmless but misleading in the audit
// log; the LLM-facing summary should show only the original aggregate.
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

// containsTopLevelAggregate returns true if expr is (or directly wraps)
// an aggregate function call. We intentionally do not recurse into
// subqueries: an aggregate inside a subquery does not bound the outer
// result set.
func containsTopLevelAggregate(expr ast.ExprNode) bool {
	switch e := expr.(type) {
	case *ast.AggregateFuncExpr:
		return true
	case *ast.FuncCallExpr:
		// Window functions (e.g. ROW_NUMBER OVER()) are not aggregates and
		// do not bound the result. A plain FuncCallExpr does not either.
		return false
	case *ast.ParenthesesExpr:
		return containsTopLevelAggregate(e.Expr)
	default:
		return false
	}
}
