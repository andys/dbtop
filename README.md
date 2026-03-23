# dbtop

A terminal-based monitoring tool for PostgreSQL and MySQL, inspired by `top`. Written in Go.

## Features

- Real-time view of active database queries
- Queries per second, connection counts, and database stats
- Sort active queries by running time
- Drill into process details: full query text, `EXPLAIN` plan, and locks held
- Keyboard-driven: arrow keys to navigate, Enter for details, `q` to quit
- Auto-detects database type from URI scheme

## Supported Databases

| Database   | URI Format                                      | Driver            |
|------------|------------------------------------------------|-------------------|
| PostgreSQL | `postgres://user:pass@host:5432/dbname`        | pgx               |
| MySQL      | `mysql://user:pass@host:3306/dbname`           | go-sql-driver/mysql |

MySQL also accepts raw DSN format: `user:pass@tcp(host:3306)/dbname`

### MySQL Notes

- Process monitoring uses `information_schema.PROCESSLIST`
- Database stats come from `SHOW GLOBAL STATUS` (InnoDB metrics)
- Lock inspection requires MySQL 8.0+ (`performance_schema.data_locks`); degrades gracefully on older versions
- `EXPLAIN` output is displayed in tabular format

## Installation

```sh
go install github.com/andys/dbtop@latest
```

## Usage

```sh
# PostgreSQL
dbtop postgres://user:pass@host:5432/dbname

# MySQL
dbtop mysql://user:pass@host:3306/dbname
```

## Keybindings

| Key        | Action              |
|------------|---------------------|
| ↑ / ↓      | Select process      |
| Enter      | Show process detail |
| q / Ctrl+C | Quit                |

## License

[MIT](LICENSE)
