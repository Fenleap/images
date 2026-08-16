package main

import (
	"fmt"
	"os"
	"strings"
)

// Connection describes where a client should connect, resolved either from
// discrete environment variables ("fields" mode) or from a single connection
// string ("dsn" mode).
//
// Parsing happens here, inside the pod, on purpose: the DSN is supplied through
// a Kubernetes Secret and must never travel back to the control plane that
// orchestrates the session. A launcher that resolves it in-process keeps the
// credential inside the container boundary, which is the property the whole
// db-tools design exists to preserve.
type Connection struct {
	Host string
	Port string
	User string

	// Password is carried ONLY when it came from a DSN, where it is the sole
	// place the credential exists — there is no separate MYSQL_PWD to fall back
	// on. It is handed to the client through the environment, never through
	// argv, because argv is world-readable via /proc/<pid>/cmdline to anything
	// else in the pod while the environment of a running process is not.
	//
	// In fields mode this stays empty and the client reads MYSQL_PWD itself.
	Password string
}

const defaultMySQLPort = "3306"

// ParseMySQLDSN understands the two connection-string dialects that reach a
// MySQL client in practice:
//
//	user:pass@tcp(host:port)/db        Go sql driver form
//	[scheme://]user:pass@host:port/db  URL form
//
// The username/password split takes the FIRST colon and the host split takes
// the LAST "@", so passwords containing ":" or "@" survive intact — the most
// common source of "works locally, fails in prod" connection bugs.
func ParseMySQLDSN(dsn string) (Connection, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return Connection{}, fmt.Errorf("connection string is empty")
	}

	var credentials, hostPort string
	if idx := strings.LastIndex(dsn, "@tcp("); idx != -1 {
		credentials = dsn[:idx]
		rest := dsn[idx+len("@tcp("):]
		end := strings.Index(rest, ")")
		if end == -1 {
			return Connection{}, fmt.Errorf("malformed DSN: missing ')' after tcp(")
		}
		hostPort = rest[:end]
	} else {
		trimmed := dsn
		if idx := strings.Index(trimmed, "://"); idx != -1 {
			trimmed = trimmed[idx+3:]
		}
		at := strings.LastIndex(trimmed, "@")
		if at == -1 {
			return Connection{}, fmt.Errorf("malformed DSN: no '@' separating credentials from host")
		}
		credentials = trimmed[:at]
		hostPort = trimmed[at+1:]
		if cut := strings.IndexAny(hostPort, "/?"); cut != -1 {
			hostPort = hostPort[:cut]
		}
	}

	user, password := credentials, ""
	if colon := strings.Index(credentials, ":"); colon != -1 {
		// First colon splits user from password, so a password containing ":"
		// keeps everything after that first one.
		user, password = credentials[:colon], credentials[colon+1:]
	}

	host, port := hostPort, defaultMySQLPort
	if colon := strings.LastIndex(hostPort, ":"); colon != -1 {
		host, port = hostPort[:colon], hostPort[colon+1:]
	}
	if host == "" {
		return Connection{}, fmt.Errorf("malformed DSN: empty host")
	}
	if port == "" {
		port = defaultMySQLPort
	}

	return Connection{Host: host, Port: port, User: user, Password: password}, nil
}

// MySQLConnection resolves the MySQL endpoint from the environment, preferring
// DB_DSN when present and otherwise reading the discrete DB_* fields.
func MySQLConnection(env func(string) string) (Connection, error) {
	if dsn := env("DB_DSN"); dsn != "" {
		return ParseMySQLDSN(dsn)
	}
	host := env("DB_HOST")
	if host == "" {
		return Connection{}, fmt.Errorf("set DB_HOST (or DB_DSN)")
	}
	port := env("DB_PORT")
	if port == "" {
		port = defaultMySQLPort
	}
	return Connection{Host: host, Port: port, User: env("DB_USER")}, nil
}

// Getenv is the default environment lookup, split out so tests can inject one.
func Getenv(key string) string { return os.Getenv(key) }

// truthy accepts the spellings a Kubernetes env var realistically carries.
func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
