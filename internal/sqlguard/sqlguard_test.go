package sqlguard

import "testing"

func TestValidate_BlocksAdversarial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		sql                   string
		wantBlockedConstructs []string
	}{
		{name: "drop table", sql: "DROP TABLE users", wantBlockedConstructs: []string{"statement_drop"}},
		{name: "delete", sql: "DELETE FROM users", wantBlockedConstructs: []string{"statement_delete"}},
		{name: "update", sql: "UPDATE users SET admin=1", wantBlockedConstructs: []string{"statement_update"}},
		{name: "insert", sql: "INSERT INTO users VALUES (1)", wantBlockedConstructs: []string{"statement_insert"}},
		{name: "truncate", sql: "TRUNCATE users", wantBlockedConstructs: []string{"statement_truncate"}},
		{name: "alter table", sql: "ALTER TABLE users ADD COLUMN x INT", wantBlockedConstructs: []string{"statement_alter"}},
		{name: "grant all", sql: "GRANT ALL ON *.* TO 'a'@'%'", wantBlockedConstructs: []string{"statement_grant"}},
		{name: "multi statement", sql: "SELECT 1; DROP TABLE users", wantBlockedConstructs: []string{"multi_statement"}},
		{name: "line comment injection", sql: "SELECT 1 -- ; DROP TABLE users\n", wantBlockedConstructs: []string{"comment"}},
		{name: "block comment injection", sql: "SELECT 1 /* */ ; DROP TABLE users", wantBlockedConstructs: []string{"comment"}},
		{name: "into outfile", sql: "SELECT * FROM users INTO OUTFILE '/tmp/x'", wantBlockedConstructs: []string{"select_into"}},
		{name: "into dumpfile", sql: "SELECT * FROM users INTO DUMPFILE '/tmp/x'", wantBlockedConstructs: []string{"select_into"}},
		{name: "for update", sql: "SELECT * FROM users FOR UPDATE", wantBlockedConstructs: []string{"for_update"}},
		{name: "call", sql: "CALL some_proc()", wantBlockedConstructs: []string{"statement_call"}},
		{name: "load data", sql: "LOAD DATA INFILE '/etc/passwd' INTO TABLE t", wantBlockedConstructs: []string{"statement_load_data"}},
		{name: "handler", sql: "HANDLER t OPEN", wantBlockedConstructs: []string{"parse_error"}},
		{name: "lock tables", sql: "LOCK TABLES users WRITE", wantBlockedConstructs: []string{"statement_lock_tables"}},
		{name: "into user variable", sql: "SELECT @@version INTO @x", wantBlockedConstructs: []string{"select_into"}},
		{name: "rename table", sql: "RENAME TABLE a TO b", wantBlockedConstructs: []string{"statement_rename_table"}},
		{name: "create table", sql: "CREATE TABLE x (id INT)", wantBlockedConstructs: []string{"statement_create"}},
		{name: "dml in subquery", sql: "SELECT 1 UNION SELECT * FROM (DELETE FROM users RETURNING *) AS t", wantBlockedConstructs: []string{"parse_error"}},
		// Additional adversarial cases:
		{name: "revoke", sql: "REVOKE SELECT ON *.* FROM 'a'@'%'", wantBlockedConstructs: []string{"statement_revoke"}},
		{name: "unlock tables", sql: "UNLOCK TABLES", wantBlockedConstructs: []string{"statement_unlock_tables"}},
		{name: "replace", sql: "REPLACE INTO users VALUES (1)", wantBlockedConstructs: []string{"statement_replace"}},
		{name: "create view", sql: "CREATE VIEW v AS SELECT * FROM users", wantBlockedConstructs: []string{"statement_create"}},
		{name: "drop database", sql: "DROP DATABASE mission_app", wantBlockedConstructs: []string{"statement_drop"}},
		{name: "set global", sql: "SET GLOBAL sql_safe_updates = 0", wantBlockedConstructs: []string{"statement_set"}},
		{name: "do sleep", sql: "DO SLEEP(5)", wantBlockedConstructs: []string{"statement_do"}},
		{name: "hash comment injection", sql: "SELECT 1 # ; DROP TABLE users", wantBlockedConstructs: []string{"comment"}},
		{name: "versioned comment injection", sql: "SELECT /*!50000 DROP TABLE users */ 1", wantBlockedConstructs: []string{"comment"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := Validate(tc.sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", tc.sql, err)
			}
			if result.OK {
				t.Fatalf("Validate(%q) = %+v, want blocked result", tc.sql, result)
			}
			if result.Reason == "" {
				t.Fatalf("Validate(%q) returned empty reason", tc.sql)
			}
			if !equalStrings(result.BlockedConstructs, tc.wantBlockedConstructs) {
				t.Fatalf(
					"BlockedConstructs = %#v, want %#v",
					result.BlockedConstructs,
					tc.wantBlockedConstructs,
				)
			}
			if result.RewrittenSQL != "" {
				t.Fatalf("RewrittenSQL = %q, want empty for blocked SQL", result.RewrittenSQL)
			}
		})
	}
}

func TestValidate_AcceptsValidQueries(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT * FROM t WHERE id=1",
		"SELECT COUNT(*) FROM t",
		"SELECT a, SUM(b) FROM t GROUP BY a",
		"SELECT * FROM t LIMIT 50",
		"WITH x AS (SELECT * FROM t) SELECT * FROM x",
		"SELECT a.x, b.y FROM a JOIN b ON a.id=b.a_id",
		"SHOW TABLES",
		"SHOW CREATE TABLE t",
		"SELECT * FROM t ORDER BY x DESC",
		"SELECT JSON_EXTRACT(payload, '$.name') FROM t",
	}

	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()

			result, err := Validate(sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", sql, err)
			}
			if !result.OK {
				t.Fatalf("Validate(%q) = %+v, want allowed result", sql, result)
			}
			if result.RewrittenSQL == "" {
				t.Fatalf("Validate(%q) returned empty rewritten SQL", sql)
			}
			if result.Reason != "" {
				t.Fatalf("Reason = %q, want empty for allowed SQL", result.Reason)
			}
			if len(result.BlockedConstructs) != 0 {
				t.Fatalf("BlockedConstructs = %#v, want empty", result.BlockedConstructs)
			}
		})
	}
}

func TestValidate_LimitInjection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		sql               string
		wantRewrittenSQL  string
		wantLimitInjected bool
	}{
		{
			name:              "inject limit",
			sql:               "SELECT * FROM t",
			wantRewrittenSQL:  "SELECT * FROM `t` LIMIT 1000",
			wantLimitInjected: true,
		},
		{
			name:              "preserve explicit limit",
			sql:               "SELECT * FROM t LIMIT 50",
			wantRewrittenSQL:  "SELECT * FROM `t` LIMIT 50",
			wantLimitInjected: false,
		},
		{
			name:              "count aggregate unchanged",
			sql:               "SELECT COUNT(*) FROM t",
			wantRewrittenSQL:  "SELECT COUNT(1) FROM `t`",
			wantLimitInjected: false,
		},
		{
			name:              "group by unchanged",
			sql:               "SELECT a, SUM(b) FROM t GROUP BY a",
			wantRewrittenSQL:  "SELECT `a`,SUM(`b`) FROM `t` GROUP BY `a`",
			wantLimitInjected: false,
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
			if !result.OK {
				t.Fatalf("Validate(%q) = %+v, want allowed result", tc.sql, result)
			}
			if result.RewrittenSQL != tc.wantRewrittenSQL {
				t.Fatalf(
					"RewrittenSQL = %q, want %q",
					result.RewrittenSQL,
					tc.wantRewrittenSQL,
				)
			}
			if result.LimitInjected != tc.wantLimitInjected {
				t.Fatalf(
					"LimitInjected = %v, want %v",
					result.LimitInjected,
					tc.wantLimitInjected,
				)
			}
		})
	}
}

func TestValidate_RewrittenSQLRoundTrips(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT * FROM t",
		"SELECT COUNT(*) FROM t",
		"WITH x AS (SELECT * FROM t) SELECT * FROM x",
		"SHOW TABLES",
	}

	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()

			result, err := Validate(sql)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", sql, err)
			}
			if !result.OK {
				t.Fatalf("Validate(%q) = %+v, want allowed result", sql, result)
			}

			revalidated, err := Validate(result.RewrittenSQL)
			if err != nil {
				t.Fatalf("Validate(%q) returned error: %v", result.RewrittenSQL, err)
			}
			if !revalidated.OK {
				t.Fatalf(
					"Validate(%q) = %+v, want allowed result",
					result.RewrittenSQL,
					revalidated,
				)
			}
		})
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
