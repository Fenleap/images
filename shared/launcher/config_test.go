package main

import (
	"strings"
	"testing"
)

func TestParseMySQLDSN(t *testing.T) {
	cases := []struct {
		name             string
		dsn              string
		host, port, user string
	}{
		{"go driver", "app:secret@tcp(db.internal:3306)/orders", "db.internal", "3306", "app"},
		{"go driver default port", "app:secret@tcp(db.internal)/orders", "db.internal", "3306", "app"},
		{"url with scheme", "mysql://app:secret@db.internal:3307/orders", "db.internal", "3307", "app"},
		{"url without scheme", "app:secret@db.internal:3307/orders", "db.internal", "3307", "app"},
		{"url without port", "mysql://app:secret@db.internal/orders", "db.internal", "3306", "app"},
		{"url with query", "mysql://app:secret@db.internal:3306/orders?tls=true", "db.internal", "3306", "app"},
		{"no database", "mysql://app:secret@db.internal:3306", "db.internal", "3306", "app"},
		// The failure mode this parser exists to avoid: a password containing
		// the same characters used as separators.
		{"password with at", "app:p@ssw0rd@tcp(db.internal:3306)/orders", "db.internal", "3306", "app"},
		{"password with colon", "app:pass:word@tcp(db.internal:3306)/orders", "db.internal", "3306", "app"},
		{"password with at, url form", "mysql://app:p@ss@db.internal:3306/orders", "db.internal", "3306", "app"},
		{"rds endpoint", "admin:x@tcp(prod.abc123.eu-west-1.rds.amazonaws.com:3306)/app",
			"prod.abc123.eu-west-1.rds.amazonaws.com", "3306", "admin"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMySQLDSN(tc.dsn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Host != tc.host || got.Port != tc.port || got.User != tc.user {
				t.Fatalf("got %+v, want host=%s port=%s user=%s", got, tc.host, tc.port, tc.user)
			}
		})
	}
}

func TestParseMySQLDSNRejects(t *testing.T) {
	for _, dsn := range []string{"", "   ", "no-at-sign-here", "app:secret@tcp(db.internal:3306"} {
		if _, err := ParseMySQLDSN(dsn); err == nil {
			t.Fatalf("expected an error for %q", dsn)
		}
	}
}

// Regression: the password lives INSIDE the DSN, so dropping it during parsing
// left the client connecting with nothing —
// "Access denied for user 'x'@'ip' (using password: NO)". Redis never showed
// this because redis-cli takes the DSN verbatim via -u.
func TestParseMySQLDSNKeepsThePassword(t *testing.T) {
	cases := []struct{ dsn, want string }{
		{"app:sup3rsecret@tcp(db:3306)/x", "sup3rsecret"},
		{"mysql://app:sup3rsecret@db:3306/x", "sup3rsecret"},
		{"app:p@ssw0rd@tcp(db:3306)/x", "p@ssw0rd"},
		{"app:pass:word@tcp(db:3306)/x", "pass:word"},
	}
	for _, tc := range cases {
		conn, err := ParseMySQLDSN(tc.dsn)
		if err != nil {
			t.Fatalf("%s: %v", tc.dsn, err)
		}
		if conn.Password != tc.want {
			t.Fatalf("%s: got password %q, want %q", tc.dsn, conn.Password, tc.want)
		}
	}
}

func TestParseMySQLDSNWithoutAPassword(t *testing.T) {
	conn, err := ParseMySQLDSN("app@tcp(db:3306)/x")
	if err != nil {
		t.Fatal(err)
	}
	if conn.User != "app" || conn.Password != "" {
		t.Fatalf("got user=%q password=%q", conn.User, conn.Password)
	}
}

func TestMySQLEnvCarriesTheDSNPassword(t *testing.T) {
	t.Setenv("MYSQL_PWD", "")
	env := mysqlEnv(Connection{Host: "h", Port: "3306", User: "u", Password: "fromdsn"})

	var found string
	for _, e := range env {
		if strings.HasPrefix(e, "MYSQL_PWD=") {
			found = strings.TrimPrefix(e, "MYSQL_PWD=")
		}
	}
	if found != "fromdsn" {
		t.Fatalf("MYSQL_PWD not set from the DSN, got %q", found)
	}
}

func TestMySQLEnvDoesNotOverrideAnExplicitPassword(t *testing.T) {
	// Fields mode already supplies MYSQL_PWD from the Secret, and an operator
	// overriding it must win over whatever a stale DSN carries.
	t.Setenv("MYSQL_PWD", "fromsecret")
	env := mysqlEnv(Connection{Host: "h", Port: "3306", User: "u", Password: "fromdsn"})
	for _, e := range env {
		if e == "MYSQL_PWD=fromdsn" {
			t.Fatal("DSN password overrode an explicitly set MYSQL_PWD")
		}
	}
}

func TestPasswordNeverReachesArgv(t *testing.T) {
	// The invariant that actually matters: /proc/<pid>/cmdline is readable by
	// anything else in the pod; a process's environment is not.
	conn := Connection{Host: "h", Port: "3306", User: "u", Password: "sup3rsecret"}
	for _, a := range mysqlArgs(conn, true, "SELECT 1") {
		if strings.Contains(a, "sup3rsecret") {
			t.Fatalf("password leaked into argv: %q", a)
		}
	}
}

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestMySQLConnectionPrefersDSN(t *testing.T) {
	conn, err := MySQLConnection(envFrom(map[string]string{
		"DB_DSN":  "app:secret@tcp(from-dsn:3307)/x",
		"DB_HOST": "from-fields",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if conn.Host != "from-dsn" || conn.Port != "3307" {
		t.Fatalf("expected the DSN to win, got %+v", conn)
	}
}

func TestMySQLConnectionFields(t *testing.T) {
	conn, err := MySQLConnection(envFrom(map[string]string{
		"DB_HOST": "db.internal", "DB_PORT": "3307", "DB_USER": "app",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if conn.Host != "db.internal" || conn.Port != "3307" || conn.User != "app" {
		t.Fatalf("got %+v", conn)
	}
}

func TestMySQLConnectionDefaultsAndErrors(t *testing.T) {
	conn, err := MySQLConnection(envFrom(map[string]string{"DB_HOST": "db"}))
	if err != nil {
		t.Fatal(err)
	}
	if conn.Port != "3306" {
		t.Fatalf("expected default port, got %q", conn.Port)
	}
	if _, err := MySQLConnection(envFrom(map[string]string{})); err == nil {
		t.Fatal("expected an error when neither DB_DSN nor DB_HOST is set")
	}
}

func TestMySQLArgsNeverCarryAPassword(t *testing.T) {
	// MYSQL_PWD is read by the client itself. If it ever reached argv it would
	// be world-readable via /proc/<pid>/cmdline inside the pod.
	args := mysqlArgs(Connection{Host: "h", Port: "3306", User: "u"}, false, "")
	for _, a := range args {
		if strings.Contains(a, "-p") && a != "-P" {
			t.Fatalf("password-ish flag in argv: %q", a)
		}
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ssl-mode=REQUIRED") {
		t.Fatalf("expected TLS to be required without a CA, got %q", joined)
	}
}

func TestMySQLArgsBatchMode(t *testing.T) {
	args := mysqlArgs(Connection{Host: "h", Port: "3306"}, true, "SELECT 1")
	joined := strings.Join(args, " ")
	for _, want := range []string{"--batch", "--quick", "--default-character-set=utf8mb4"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %q", want, joined)
		}
	}
	if strings.Contains(joined, "--raw") {
		t.Fatal("--raw would break the one-row-per-line guarantee")
	}
	// The statement must be its own argv entry, never spliced into a string.
	if args[len(args)-2] != "-e" || args[len(args)-1] != "SELECT 1" {
		t.Fatalf("statement not passed as a discrete argument: %q", args)
	}
}

func TestMySQLArgsOmitsUserWhenUnset(t *testing.T) {
	args := mysqlArgs(Connection{Host: "h", Port: "3306"}, false, "")
	if strings.Contains(strings.Join(args, " "), "-u") {
		t.Fatal("expected no -u when no username is configured")
	}
}

func TestRedisArgsFieldsMode(t *testing.T) {
	args := redisArgs(envFrom(map[string]string{
		"REDIS_HOST": "cache", "REDIS_PORT": "6380", "REDIS_USER": "app",
	}), nil)
	joined := strings.Join(args, " ")
	for _, want := range []string{"-h cache", "-p 6380", "--user app"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
}

func TestRedisArgsDSNPassthrough(t *testing.T) {
	args := redisArgs(envFrom(map[string]string{"REDIS_DSN": "redis://user:pw@cache:6379"}), nil)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-u redis://user:pw@cache:6379") {
		t.Fatalf("expected the DSN to be forwarded verbatim, got %q", joined)
	}
}

func TestRedisArgsTLSAndCluster(t *testing.T) {
	args := redisArgs(envFrom(map[string]string{
		"REDIS_HOST": "cache", "REDIS_TLS": "true", "REDIS_CLUSTER": "yes",
	}), nil)
	joined := strings.Join(args, " ")
	// No CA is mounted in the test environment, so it must fall back to
	// encrypted-but-unverified rather than failing to connect.
	for _, want := range []string{"--tls", "--insecure", "-c"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %q", want, joined)
		}
	}
}

func TestRedisArgsKeepsCommandTokensSeparate(t *testing.T) {
	args := redisArgs(envFrom(map[string]string{"REDIS_HOST": "cache"}),
		[]string{"GET", "a; rm -rf /"})
	last := args[len(args)-1]
	if last != "a; rm -rf /" {
		t.Fatalf("token was altered: %q", last)
	}
}

func TestParseClientFlags(t *testing.T) {
	batch, stmt, err := parseClientFlags([]string{"--batch", "--statement", "SELECT 1"})
	if err != nil || !batch || stmt != "SELECT 1" {
		t.Fatalf("got batch=%v stmt=%q err=%v", batch, stmt, err)
	}

	// A statement starting with "-" must not be mistaken for a flag.
	_, stmt, err = parseClientFlags([]string{"--statement", "-- not a flag"})
	if err != nil || stmt != "-- not a flag" {
		t.Fatalf("got stmt=%q err=%v", stmt, err)
	}

	// Anything unrecognised is refused rather than forwarded to the client,
	// so a caller cannot smuggle in client flags such as --pager.
	if _, _, err := parseClientFlags([]string{"--pager=sh -c id"}); err == nil {
		t.Fatal("expected unknown arguments to be rejected")
	}
	if _, _, err := parseClientFlags([]string{"--statement"}); err == nil {
		t.Fatal("expected an error for a missing --statement value")
	}
}

func TestTruthy(t *testing.T) {
	for _, yes := range []string{"1", "true", "TRUE", " yes ", "on"} {
		if !truthy(yes) {
			t.Fatalf("%q should be truthy", yes)
		}
	}
	for _, no := range []string{"", "0", "false", "off", "maybe"} {
		if truthy(no) {
			t.Fatalf("%q should not be truthy", no)
		}
	}
}
