package sqlguard

import (
	"strings"
	"testing"
)

func TestValidate_BlocksAdversarial(t *testing.T) {
	t.Parallel()

	// 32 cases covering DDL, DML, multi-statement, INTO OUTFILE/DUMPFILE,
	// stored procedures, transaction control, admin, session mutation,
	// FOR UPDATE, LOAD DATA, GRANT, empty/garbage, and version-hint
	// comment injection. Any of these reaching the MySQL gateway would
	// defeat the read-only safety posture.
	cases := map[string]string{
		// DDL
		"drop table":      "DROP TABLE users",
		"create table":    "CREATE TABLE foo (id INT)",
		"alter table":     "ALTER TABLE foo ADD COLUMN bar INT",
		"truncate":        "TRUNCATE TABLE users",
		"rename table":    "RENAME TABLE old TO new",
		"create index":    "CREATE INDEX idx ON foo(id)",
		"drop database":   "DROP DATABASE mydb",
		"create database": "CREATE DATABASE mydb",

		// DML
		"update":  "UPDATE foo SET x = 1",
		"delete":  "DELETE FROM foo",
		"insert":  "INSERT INTO foo VALUES (1)",
		"replace": "REPLACE INTO foo VALUES (1)",

		// Multi-statement
		"select then drop":   "SELECT 1; DROP TABLE users",
		"select then select": "SELECT 1; SELECT 2",
		"trailing drop":      "SELECT 1; /* comment */ DROP TABLE x;",

		// INTO OUTFILE / DUMPFILE / @var
		"into outfile":  "SELECT * FROM foo INTO OUTFILE '/tmp/x'",
		"into dumpfile": "SELECT * FROM foo INTO DUMPFILE '/tmp/x'",
		"into var":      "SELECT id FROM foo INTO @x",

		// Procedure-style / admin misuse
		"call":              "CALL my_proc()",
		"do sleep":          "DO SLEEP(5)",
		"start transaction": "START TRANSACTION",
		"commit":            "COMMIT",
		"rollback":          "ROLLBACK",
		"set global":        "SET GLOBAL max_connections = 10",
		"kill":              "KILL 1",
		"flush":             "FLUSH PRIVILEGES",
		"grant":             "GRANT SELECT ON foo TO 'user'@'%'",
		"load data":         "LOAD DATA INFILE '/tmp/x' INTO TABLE foo",

		// Read/write locks on SELECT
		"select for update": "SELECT * FROM foo FOR UPDATE",

		// Garbage / empty
		"empty":        "",
		"whitespace":   "   \n\t  ",
		"nonsense":     "not a real SQL statement",
		"only comment": "-- just a comment",
	}

	for name, sql := range cases {
		sql := sql
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := Validate(sql)
			if err == nil {
				t.Fatalf(
					"Validate(%q) = %+v, want error",
					sql,
					result,
				)
			}
		})
	}
}

func TestValidate_CommentInjectionNeutralized(t *testing.T) {
	t.Parallel()

	// Comment-wrapped payloads must not leak into the rewritten SQL.
	// These inputs parse as plain SELECT 1 (the comments are stripped on
	// restore), so they pass validation — but the rewritten SQL must not
	// contain the adversarial payload.
	cases := map[string]string{
		"block comment carrying drop": "SELECT 1 /*; DROP TABLE x; */",
		"line comment carrying drop":  "SELECT 1 -- ; DROP TABLE x",
	}

	for name, sql := range cases {
		sql := sql
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := Validate(sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", sql, err)
			}
			upper := strings.ToUpper(result.RewrittenSQL)
			if strings.Contains(upper, "DROP") {
				t.Fatalf(
					"rewritten SQL %q leaked DROP payload",
					result.RewrittenSQL,
				)
			}
		})
	}
}

func TestValidate_VersionHintCommentRejected(t *testing.T) {
	t.Parallel()

	// `/*! ... */` is MySQL's conditional-execution comment. Anything
	// inside runs when the server version matches. An UPDATE hidden
	// inside must not slip through. tidb's parser interprets these, so
	// the UPDATE becomes part of the AST and our allowlist rejects it.
	sql := "SELECT /*!UPDATE foo SET x=1*/ 1"
	if _, err := Validate(sql); err == nil {
		t.Fatalf("Validate(%q) accepted a version-hint UPDATE", sql)
	}
}

func TestValidate_AcceptsValidSelects(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"select literal":    "SELECT 1",
		"select star":       "SELECT * FROM users",
		"select columns":    "SELECT id, name FROM users WHERE active = 1",
		"join":              "SELECT u.id FROM users u JOIN posts p ON p.user_id = u.id",
		"group by":          "SELECT name, SUM(amount) FROM orders GROUP BY name",
		"with cte":          "WITH cte AS (SELECT id FROM users) SELECT * FROM cte",
		"union":             "SELECT id FROM a UNION SELECT id FROM b",
		"order by":          "SELECT * FROM users ORDER BY id DESC",
		"existing limit":    "SELECT * FROM users LIMIT 10",
		"aggregate only":    "SELECT AVG(price) FROM products",
		"subquery":          "SELECT id FROM users WHERE id IN (SELECT user_id FROM sessions)",
		"for share":         "SELECT * FROM users FOR SHARE",
		"lock in share":     "SELECT * FROM users LOCK IN SHARE MODE",
		"count star":        "SELECT COUNT(*) FROM users",
		"having":            "SELECT name, COUNT(*) c FROM users GROUP BY name HAVING c > 1",
		"distinct":          "SELECT DISTINCT country FROM users",
		"case expression":   "SELECT CASE WHEN age > 18 THEN 'adult' ELSE 'minor' END FROM users",
		"parameter literal": "SELECT * FROM users WHERE id = 42",
	}

	for name, sql := range cases {
		sql := sql
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			result, err := Validate(sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", sql, err)
			}
			if result.RewrittenSQL == "" {
				t.Fatalf("RewrittenSQL is empty for %q", sql)
			}
		})
	}
}

func TestValidate_LimitInjection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		sql                 string
		wantLimitInjected   bool
		wantSQLContainsLike string // substring that must appear in rewritten SQL
	}{
		{
			name:                "plain select gets limit",
			sql:                 "SELECT * FROM users",
			wantLimitInjected:   true,
			wantSQLContainsLike: "LIMIT 1000",
		},
		{
			name:                "explicit limit is preserved",
			sql:                 "SELECT * FROM users LIMIT 10",
			wantLimitInjected:   false,
			wantSQLContainsLike: "LIMIT 10",
		},
		{
			name:              "aggregate has no limit injected",
			sql:               "SELECT COUNT(*) FROM users",
			wantLimitInjected: false,
		},
		{
			name:              "group by has no limit injected",
			sql:               "SELECT name, SUM(amount) FROM orders GROUP BY name",
			wantLimitInjected: false,
		},
		{
			name:              "avg aggregate no limit",
			sql:               "SELECT AVG(price) FROM products",
			wantLimitInjected: false,
		},
		{
			name:                "union without limit gets one",
			sql:                 "SELECT id FROM a UNION SELECT id FROM b",
			wantLimitInjected:   true,
			wantSQLContainsLike: "LIMIT 1000",
		},
		{
			name:              "union with outer limit keeps it",
			sql:               "(SELECT id FROM a) UNION (SELECT id FROM b) LIMIT 50",
			wantLimitInjected: false,
		},
		{
			name:                "subquery does not count as aggregation",
			sql:                 "SELECT id FROM users WHERE id IN (SELECT user_id FROM sessions)",
			wantLimitInjected:   true,
			wantSQLContainsLike: "LIMIT 1000",
		},
		{
			name:                "cte without limit gets one",
			sql:                 "WITH cte AS (SELECT id FROM users) SELECT * FROM cte",
			wantLimitInjected:   true,
			wantSQLContainsLike: "LIMIT 1000",
		},
		{
			name:                "order by no limit gets one",
			sql:                 "SELECT * FROM users ORDER BY id DESC",
			wantLimitInjected:   true,
			wantSQLContainsLike: "LIMIT 1000",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := Validate(tc.sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", tc.sql, err)
			}
			if result.LimitInjected != tc.wantLimitInjected {
				t.Fatalf(
					"LimitInjected = %v, want %v (rewritten=%q)",
					result.LimitInjected,
					tc.wantLimitInjected,
					result.RewrittenSQL,
				)
			}
			if tc.wantSQLContainsLike != "" &&
				!strings.Contains(
					strings.ToUpper(result.RewrittenSQL),
					tc.wantSQLContainsLike,
				) {
				t.Fatalf(
					"rewritten SQL %q missing substring %q",
					result.RewrittenSQL,
					tc.wantSQLContainsLike,
				)
			}
		})
	}
}

func TestValidate_RewrittenSQLExecutes(t *testing.T) {
	t.Parallel()

	// The rewritten SQL must itself be parseable by the same parser —
	// otherwise we'd emit something MySQL rejects and leak the error as
	// a DB failure instead of a guard rejection. This catches any
	// mis-restoration, especially around LIMIT injection corner cases.
	cases := []string{
		"SELECT 1",
		"SELECT * FROM users",
		"SELECT COUNT(*) FROM users",
		"WITH cte AS (SELECT 1) SELECT * FROM cte",
		"SELECT id FROM a UNION SELECT id FROM b",
		"SELECT * FROM users ORDER BY id LIMIT 5",
	}

	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			result, err := Validate(sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", sql, err)
			}
			// Re-validating the rewritten output should always succeed.
			if _, err := Validate(result.RewrittenSQL); err != nil {
				t.Fatalf(
					"rewritten SQL %q failed re-validation: %v",
					result.RewrittenSQL,
					err,
				)
			}
		})
	}
}
