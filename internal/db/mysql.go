package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLDB implements Database for MySQL using database/sql.
type MySQLDB struct {
	db *sql.DB
}

// parseMySQLURI converts a mysql:// URI to a Go MySQL DSN.
// mysql://user:pass@host:port/dbname -> user:pass@tcp(host:port)/dbname
func parseMySQLURI(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("parse mysql URI: %w", err)
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}

	user := u.User.Username()
	pass, _ := u.User.Password()

	dbname := strings.TrimPrefix(u.Path, "/")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, pass, host, port, dbname)
	return dsn, nil
}

func newMySQLDB(ctx context.Context, uri string) (*MySQLDB, error) {
	var dsn string
	if strings.HasPrefix(uri, "mysql://") {
		var err error
		dsn, err = parseMySQLURI(uri)
		if err != nil {
			return nil, err
		}
	} else {
		dsn = uri // assume raw DSN
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &MySQLDB{db: db}, nil
}

func (m *MySQLDB) DBType() string { return "MySQL" }

func (m *MySQLDB) Close(_ context.Context) error {
	return m.db.Close()
}

func (m *MySQLDB) GetVersion(ctx context.Context) (string, error) {
	var version string
	err := m.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	return version, err
}

const mysqlQueryProcesses = `
SELECT ID,
       COALESCE(INFO, '') AS query_text,
       COALESCE(COMMAND, '') AS command,
       COALESCE(USER, '') AS user,
       COALESCE(DB, '') AS db_name,
       COALESCE(TIME, 0) AS time_secs
FROM information_schema.PROCESSLIST
WHERE ID != CONNECTION_ID()
ORDER BY TIME DESC
`

func (m *MySQLDB) GetProcesses(ctx context.Context) ([]Process, error) {
	rows, err := m.db.QueryContext(ctx, mysqlQueryProcesses)
	if err != nil {
		return nil, fmt.Errorf("mysql query processes: %w", err)
	}
	defer rows.Close()

	var procs []Process
	for rows.Next() {
		var p Process
		err := rows.Scan(&p.PID, &p.Query, &p.State, &p.User, &p.Database, &p.QuerySeconds)
		if err != nil {
			return nil, fmt.Errorf("mysql scan process: %w", err)
		}
		p.XactSeconds = p.QuerySeconds // MySQL doesn't separate these
		procs = append(procs, p)
	}
	return procs, rows.Err()
}

func (m *MySQLDB) GetStats(ctx context.Context) (Stats, error) {
	var s Stats

	// Get database name
	_ = m.db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&s.DatabaseName)

	// Get global status variables
	rows, err := m.db.QueryContext(ctx, `SHOW GLOBAL STATUS WHERE Variable_name IN (
		'Com_commit', 'Com_rollback',
		'Innodb_buffer_pool_reads', 'Innodb_buffer_pool_read_requests',
		'Innodb_rows_read', 'Innodb_rows_inserted'
	)`)
	if err != nil {
		return s, fmt.Errorf("mysql global status: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			continue
		}
		v, _ := strconv.ParseInt(value, 10, 64)
		switch name {
		case "Com_commit":
			s.XactCommit = v
		case "Com_rollback":
			s.XactRollback = v
		case "Innodb_buffer_pool_reads":
			s.BlksRead = v
		case "Innodb_buffer_pool_read_requests":
			s.BlksHit = v
		case "Innodb_rows_read":
			s.TupReturned = v
		case "Innodb_rows_inserted":
			s.TupFetched = v
		}
	}
	return s, rows.Err()
}

func (m *MySQLDB) Explain(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "No query to explain.", nil
	}

	rows, err := m.db.QueryContext(ctx, "EXPLAIN "+query)
	if err != nil {
		return fmt.Sprintf("EXPLAIN error: %v", err), nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return fmt.Sprintf("EXPLAIN error: %v", err), nil
	}

	var lines []string
	lines = append(lines, strings.Join(cols, "\t"))

	values := make([]sql.NullString, len(cols))
	scanArgs := make([]interface{}, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Sprintf("EXPLAIN error: %v", err), nil
		}
		var parts []string
		for _, v := range values {
			if v.Valid {
				parts = append(parts, v.String)
			} else {
				parts = append(parts, "NULL")
			}
		}
		lines = append(lines, strings.Join(parts, "\t"))
	}
	return strings.Join(lines, "\n"), nil
}

// mysqlQueryLocks uses performance_schema.data_locks (MySQL 8.0+).
const mysqlQueryLocks = `
SELECT COALESCE(OBJECT_SCHEMA, '') AS schema_name,
       COALESCE(OBJECT_NAME, '') AS table_name,
       COALESCE(LOCK_TYPE, '') AS lock_type,
       COALESCE(LOCK_MODE, '') AS lock_mode,
       COALESCE(LOCK_STATUS, '') AS lock_status
FROM performance_schema.data_locks
WHERE THREAD_ID = (
    SELECT THREAD_ID FROM performance_schema.threads WHERE PROCESSLIST_ID = ?
)
`

func (m *MySQLDB) GetLocks(ctx context.Context, pid int) ([]Lock, error) {
	rows, err := m.db.QueryContext(ctx, mysqlQueryLocks, pid)
	if err != nil {
		// performance_schema may not be available; degrade gracefully
		return nil, nil
	}
	defer rows.Close()

	var locks []Lock
	for rows.Next() {
		var l Lock
		var lockStatus string
		err := rows.Scan(&l.Schema, &l.Table, &l.Index, &l.Mode, &lockStatus)
		if err != nil {
			return nil, fmt.Errorf("mysql scan lock: %w", err)
		}
		l.Granted = lockStatus == "GRANTED"
		locks = append(locks, l)
	}
	return locks, rows.Err()
}
