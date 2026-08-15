// Command dbclient is the only executable in the Fenwave db-tools images.
//
// # Why it exists
//
// A database console image wants two things that normally require a shell:
// keeping the container alive while no session is running, and expanding
// Secret-backed environment variables into client flags at exec time. Shipping
// /bin/sh to get them is what makes the usual "mysql client in a pod" image
// dangerous, because the MySQL client's own `\!` command shells out — so any
// user who can reach the console can read the database password out of the
// environment and get an interactive shell in the pod.
//
// dbclient provides exactly those two capabilities and nothing else, so the
// image can ship without a shell at all. With no /bin/sh present, `\! sh`
// has nothing to spawn.
//
// What it deliberately does NOT do
//
//   - It never spawns a shell, and never interprets its arguments as a command.
//   - It never places a password on a command line; both clients read their own
//     password env var, so credentials stay out of /proc/<pid>/cmdline.
//   - It exposes no "run arbitrary program" mode. The subcommand set is fixed.
//
// Usage
//
//	dbclient idle                              keep the container alive
//	dbclient mysql                             interactive MySQL session
//	dbclient mysql --batch --statement <SQL>    one statement, TSV on stdout
//	dbclient redis                             interactive Redis session
//	dbclient redis -- GET mykey                 one command, raw output
//
// dbclient replaces itself with the client via execve, so the client inherits
// PID 1 and the TTY. That makes `kubectl attach` work, and means the pod exits
// when the session ends.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

const (
	mysqlBinary = "/usr/bin/mysql"
	redisBinary = "/usr/bin/redis-cli"
	// caFile is where the orchestrator mounts a private/RDS CA bundle.
	caFile = "/etc/db-tools/ca.pem"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "idle":
		idle()
		return
	case "mysql":
		err = runMySQL(os.Args[2:])
	case "redis":
		err = runRedis(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println(version)
		return
	case "help", "--help", "-h":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "dbclient: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "dbclient: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dbclient - launcher for the Fenwave db-tools images

  dbclient idle                             keep the container alive
  dbclient mysql                            interactive MySQL session
  dbclient mysql --batch --statement <SQL>  run one statement, TSV on stdout
  dbclient redis                            interactive Redis session
  dbclient redis -- GET mykey               run one command, raw output

Connection comes from the environment:
  MySQL  DB_DSN | DB_HOST DB_PORT DB_USER, password via MYSQL_PWD
  Redis  REDIS_DSN | REDIS_HOST REDIS_PORT REDIS_USER, password via REDISCLI_AUTH
         REDIS_TLS=true, REDIS_CLUSTER=true
A CA bundle at /etc/db-tools/ca.pem, when present, enables certificate
verification automatically.
`)
}

// idle blocks until the container is asked to stop. It replaces `sleep
// infinity`, which a shell-less image cannot run, and unlike `sleep` it exits
// promptly on SIGTERM so pod deletion does not wait for the grace period.
func idle() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
}

func hasCA() bool {
	info, err := os.Stat(caFile)
	return err == nil && !info.IsDir()
}

// mysqlArgs builds the MySQL client invocation.
//
// TLS: --ssl-ca alone would NOT verify the server certificate, because the
// client's default ssl-mode is PREFERRED. VERIFY_CA is what actually validates
// an RDS endpoint against the mounted bundle; without a bundle we still require
// encryption, we just cannot authenticate the peer.
func mysqlArgs(conn Connection, batch bool, statement string) []string {
	args := []string{"mysql", "-h", conn.Host, "-P", conn.Port}
	if conn.User != "" {
		args = append(args, "-u", conn.User)
	}
	if hasCA() {
		args = append(args, "--ssl-ca="+caFile, "--ssl-mode=VERIFY_CA")
	} else {
		args = append(args, "--ssl-mode=REQUIRED")
	}
	if batch {
		// --batch emits tab-separated rows and escapes tab/newline/backslash
		// inside values, so every row stays on one line. --raw would disable
		// that escaping and a single multi-line value would corrupt the rest
		// of the output. --quick streams rows instead of buffering the whole
		// result set in the client.
		args = append(args, "--batch", "--quick", "--default-character-set=utf8mb4")
	}
	if statement != "" {
		args = append(args, "-e", statement)
	}
	return args
}

func runMySQL(argv []string) error {
	batch, statement, err := parseClientFlags(argv)
	if err != nil {
		return err
	}
	conn, err := MySQLConnection(Getenv)
	if err != nil {
		return err
	}
	return exec(mysqlBinary, mysqlArgs(conn, batch, statement))
}

// redisArgs builds the redis-cli invocation. A DSN is passed straight through,
// since redis-cli speaks redis:// natively and there is nothing to parse.
func redisArgs(env func(string) string, passthrough []string) []string {
	args := []string{"redis-cli"}
	if dsn := env("REDIS_DSN"); dsn != "" {
		args = append(args, "-u", dsn)
	} else {
		host := env("REDIS_HOST")
		if host == "" {
			host = "127.0.0.1"
		}
		port := env("REDIS_PORT")
		if port == "" {
			port = "6379"
		}
		args = append(args, "-h", host, "-p", port)
		if user := env("REDIS_USER"); user != "" {
			args = append(args, "--user", user)
		}
	}
	if truthy(env("REDIS_TLS")) {
		args = append(args, "--tls")
		// redis-cli loads no CA of its own. With a bundle we verify against it;
		// without one we stay encrypted but unauthenticated, which is the only
		// way to reach an in-VPC ElastiCache endpoint that presents a cert the
		// image has no root for.
		if hasCA() {
			args = append(args, "--cacert", caFile)
		} else {
			args = append(args, "--insecure")
		}
	}
	if truthy(env("REDIS_CLUSTER")) {
		args = append(args, "-c")
	}
	return append(args, passthrough...)
}

func runRedis(argv []string) error {
	// Everything after "--" is a Redis command, forwarded as separate argv
	// entries. It is never concatenated into a string, so quoting, ";" and
	// "$(...)" in a key name are inert.
	passthrough := argv
	for i, a := range argv {
		if a == "--" {
			passthrough = argv[i+1:]
			break
		}
	}
	return exec(redisBinary, redisArgs(Getenv, passthrough))
}

// parseClientFlags reads the launcher's own flags. It is intentionally a hand
// written loop rather than the flag package: it must accept a statement that
// begins with "-" without treating it as a flag, and must reject anything it
// does not recognise instead of forwarding it to the client.
func parseClientFlags(argv []string) (batch bool, statement string, err error) {
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--batch":
			batch = true
		case "--statement":
			if i+1 >= len(argv) {
				return false, "", fmt.Errorf("--statement requires a value")
			}
			i++
			statement = argv[i]
		default:
			return false, "", fmt.Errorf("unsupported argument %q", argv[i])
		}
	}
	return batch, statement, nil
}

// exec replaces this process with the client, so the client keeps PID 1, the
// TTY, and the signal handling. No shell is involved and no child is spawned.
func exec(binary string, args []string) error {
	if _, statErr := os.Stat(binary); statErr != nil {
		return fmt.Errorf("client %s is not present in this image", binary)
	}
	return syscall.Exec(binary, args, os.Environ())
}
