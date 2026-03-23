package db

import (
	"context"
	"fmt"
	"strings"
)

// Database is the interface that both PostgreSQL and MySQL implementations satisfy.
type Database interface {
	Close(ctx context.Context) error
	GetVersion(ctx context.Context) (string, error)
	GetProcesses(ctx context.Context) ([]Process, error)
	GetStats(ctx context.Context) (Stats, error)
	Explain(ctx context.Context, query string) (string, error)
	GetLocks(ctx context.Context, pid int) ([]Lock, error)
	DBType() string
}

// Process represents a running database backend process.
type Process struct {
	PID          int
	Query        string
	State        string
	User         string
	Database     string
	XactSeconds  int64 // seconds since transaction start
	QuerySeconds int64 // seconds since query start
	Locks        int
}

// Stats holds database-level statistics.
type Stats struct {
	DatabaseName  string
	QueriesPerSec float64
	TotalConns    int
	ActiveConns   int
	XactCommit    int64
	XactRollback  int64
	BlksRead      int64
	BlksHit       int64
	TupReturned   int64
	TupFetched    int64
}

// Lock represents a lock held by a process.
type Lock struct {
	Database string
	Schema   string
	Table    string
	Index    string
	Mode     string
	Granted  bool
}

// NewDatabase creates the appropriate Database implementation based on the URI scheme.
// Supports postgres://, postgresql://, and mysql:// URIs.
// Also supports raw MySQL DSN format (user:pass@tcp(host:port)/db).
func NewDatabase(ctx context.Context, uri string) (Database, error) {
	switch {
	case strings.HasPrefix(uri, "postgres://"), strings.HasPrefix(uri, "postgresql://"):
		return newPostgresDB(ctx, uri)
	case strings.HasPrefix(uri, "mysql://"):
		return newMySQLDB(ctx, uri)
	default:
		// Try as MySQL DSN if it contains @ (common DSN pattern)
		if strings.Contains(uri, "@") {
			return newMySQLDB(ctx, uri)
		}
		return nil, fmt.Errorf("unsupported database URI scheme: %s", uri)
	}
}
