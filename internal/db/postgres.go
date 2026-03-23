package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// PostgresDB implements Database for PostgreSQL using pgx.
type PostgresDB struct {
	conn *pgx.Conn
}

func newPostgresDB(ctx context.Context, uri string) (*PostgresDB, error) {
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	return &PostgresDB{conn: conn}, nil
}

func (p *PostgresDB) DBType() string { return "PostgreSQL" }

func (p *PostgresDB) Close(ctx context.Context) error {
	return p.conn.Close(ctx)
}

func (p *PostgresDB) GetVersion(ctx context.Context) (string, error) {
	var version string
	err := p.conn.QueryRow(ctx, "SHOW server_version").Scan(&version)
	return version, err
}

const pgQueryProcesses = `
WITH lock_activity AS (
    SELECT pid, count(*) AS lock_count
    FROM pg_locks
    WHERE relation IS NOT NULL
    GROUP BY pid
)
SELECT a.pid,
       COALESCE(a.query, '') AS query,
       COALESCE(a.state, '') AS state,
       COALESCE(a.usename, '') AS usename,
       COALESCE(a.datname, '') AS datname,
       COALESCE(extract(EPOCH FROM age(clock_timestamp(), a.xact_start))::BIGINT, 0) AS xact_secs,
       COALESCE(extract(EPOCH FROM age(clock_timestamp(), a.query_start))::BIGINT, 0) AS query_secs,
       COALESCE(b.lock_count, 0) AS lock_count
FROM pg_stat_activity a
LEFT OUTER JOIN lock_activity b ON a.pid = b.pid
WHERE a.pid != pg_backend_pid()
ORDER BY query_secs DESC;
`

func (p *PostgresDB) GetProcesses(ctx context.Context) ([]Process, error) {
	rows, err := p.conn.Query(ctx, pgQueryProcesses)
	if err != nil {
		return nil, fmt.Errorf("query processes: %w", err)
	}
	defer rows.Close()

	var procs []Process
	for rows.Next() {
		var proc Process
		err := rows.Scan(&proc.PID, &proc.Query, &proc.State, &proc.User, &proc.Database,
			&proc.XactSeconds, &proc.QuerySeconds, &proc.Locks)
		if err != nil {
			return nil, fmt.Errorf("scan process: %w", err)
		}
		procs = append(procs, proc)
	}
	return procs, rows.Err()
}

const pgQueryStats = `
SELECT COALESCE(datname, ''),
       COALESCE(xact_commit, 0),
       COALESCE(xact_rollback, 0),
       COALESCE(blks_read, 0),
       COALESCE(blks_hit, 0),
       COALESCE(tup_returned, 0),
       COALESCE(tup_fetched, 0)
FROM pg_stat_database
WHERE datname = current_database();
`

func (p *PostgresDB) GetStats(ctx context.Context) (Stats, error) {
	var s Stats
	err := p.conn.QueryRow(ctx, pgQueryStats).Scan(
		&s.DatabaseName,
		&s.XactCommit,
		&s.XactRollback,
		&s.BlksRead,
		&s.BlksHit,
		&s.TupReturned,
		&s.TupFetched,
	)
	if err != nil {
		return Stats{}, fmt.Errorf("query stats: %w", err)
	}
	return s, nil
}

func (p *PostgresDB) Explain(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "No query to explain.", nil
	}

	tx, err := p.conn.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, "EXPLAIN "+query)
	if err != nil {
		return fmt.Sprintf("EXPLAIN error: %v", err), nil
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return fmt.Sprintf("EXPLAIN error: %v", err), nil
	}
	return strings.Join(lines, "\n"), nil
}

const pgQueryLocks = `
SELECT COALESCE(d.datname, '') AS database,
       COALESCE(nsp.nspname, '') AS schema,
       COALESCE(r.relname, '') AS table_name,
       COALESCE(i.relname, '') AS index_name,
       l.mode,
       l.granted
FROM pg_locks l
JOIN pg_stat_activity a ON a.pid = l.pid
LEFT JOIN pg_class r ON l.relation = r.oid AND r.relkind = 'r'
LEFT JOIN pg_class i ON l.relation = i.oid AND i.relkind = 'i'
LEFT JOIN pg_namespace nsp ON COALESCE(r.relnamespace, i.relnamespace) = nsp.oid
LEFT JOIN pg_database d ON l.database = d.oid
WHERE l.pid = $1
  AND l.relation IS NOT NULL;
`

func (p *PostgresDB) GetLocks(ctx context.Context, pid int) ([]Lock, error) {
	rows, err := p.conn.Query(ctx, pgQueryLocks, pid)
	if err != nil {
		return nil, fmt.Errorf("query locks: %w", err)
	}
	defer rows.Close()

	var locks []Lock
	for rows.Next() {
		var l Lock
		err := rows.Scan(&l.Database, &l.Schema, &l.Table, &l.Index, &l.Mode, &l.Granted)
		if err != nil {
			return nil, fmt.Errorf("scan lock: %w", err)
		}
		locks = append(locks, l)
	}
	return locks, rows.Err()
}
