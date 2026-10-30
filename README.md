# sqlx

[![Go Reference](https://pkg.go.dev/badge/github.com/gookit/sqlx.svg)](https://pkg.go.dev/github.com/gookit/sqlx)
[![Go version](https://img.shields.io/github/go-mod/go-version/gookit/sqlx?style=flat-square)](go.mod)
[![Actions Status](https://github.com/gookit/sqlx/actions/workflows/go.yml/badge.svg)](https://github.com/gookit/sqlx/actions)
[![License](https://img.shields.io/badge/license-MIT-red.svg)](LICENSE)

`sqlx` adds struct scanning and named parameters on top of Go's `database/sql`. It is a
maintained fork of [jmoiron/sqlx](https://github.com/jmoiron/sqlx) (MIT, the original
copyright is kept in [LICENSE](LICENSE)); import it as `github.com/gookit/sqlx`.

> [中文说明](README.zh-CN.md)

The `sqlx` versions of `sql.DB`, `sql.Tx`, `sql.Stmt` and `sql.Conn` leave the underlying
interfaces untouched, so they are a superset of the standard ones and existing
`database/sql` code keeps working.

## Features

- Scan a row into a struct (embedded structs included), a map or a slice
- Named parameters for a struct or a map, and batch insert for a slice of them
- `Get`, `Select`, `StructScan`, `SliceScan`, `MapScan` helpers
- Works on `*sqlx.DB`, `*sqlx.Tx` and `*sqlx.Conn`, with `...Context` variants
- Custom field mapping (`NameMapper`, `db` tags) and an unsafe mode that tolerates
  destination structs with fewer fields
- No cgo in the test suite: it runs on the pure Go `modernc.org/sqlite` driver

## Install

```shell
go get github.com/gookit/sqlx
```

## Quick start

```go
package main

import (
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gookit/sqlx"
)

type Person struct {
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
	Email     string
}

func main() {
	// Connect pings the database; use sqlx.Open for sql.Open semantics
	db, err := sqlx.Connect("mysql", "user:pass@tcp(127.0.0.1:3306)/db?parseTime=true")
	if err != nil {
		log.Fatal(err) // a password in the message is masked
	}
	defer db.Close()

	db.MustExec("CREATE TABLE IF NOT EXISTS person (first_name text, last_name text, email text)")

	// a transaction with named parameters
	tx := db.MustBegin()
	tx.NamedExec("INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)",
		&Person{"Jason", "Moiron", "jmoiron@jmoiron.net"})
	tx.Commit()

	// many rows into a slice
	people := []Person{}
	if err := db.Select(&people, "SELECT * FROM person ORDER BY first_name"); err != nil {
		log.Fatal(err)
	}

	// one row into a struct; sql.ErrNoRows when the query returns nothing
	jason := Person{}
	if err := db.Get(&jason, "SELECT * FROM person WHERE first_name = ?", "Jason"); err != nil {
		log.Fatal(err)
	}

	// batch insert: a slice of structs or of maps
	_, err = db.NamedExec(
		"INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)",
		[]Person{
			{"Ardie", "Savea", "asavea@ab.co.nz"},
			{"Ngani", "Laumape", "nlaumape@ab.co.nz"},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// iterate rows into a reusable struct
	rows, err := db.Queryx("SELECT * FROM person")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		p := Person{}
		if err := rows.StructScan(&p); err != nil {
			log.Fatal(err)
		}
	}
}
```

## Drivers and bind types

| bind type | driver names |
| --- | --- |
| `QUESTION` (`?`) | `mysql`, `sqlite`, `sqlite3`, `nrmysql`, `nrsqlite3` |
| `DOLLAR` (`$1`) | `postgres`, `pgx`, `pgx/v4`, `pgx/v5`, `pq-timeouts`, `cloudsqlpostgres`, `ql`, `nrpostgres`, `cockroach` |
| `NAMED` (`:arg1`) | `oci8`, `ora`, `goracle`, `godror` |
| `AT` (`@p1`) | `sqlserver`, `azuresql` |

- `sqlx.BindType(driverName)` returns the bind type of a driver.
- `sqlx.BindDriver(driverName, bindType)` registers a driver that is not listed above.
- `db.Rebind(query)` converts a `?` query to the bindvar of the driver.
- `sqlx.In(query, args...)` expands a slice to the right number of `?` placeholders. It
  always works on `?`, whatever the driver: write `IN (?)` and rebind the result for
  PostgreSQL.

## Differences from upstream

- The module path is `github.com/gookit/sqlx`; the API stays compatible with
  `jmoiron/sqlx`.
- The pure Go `modernc.org/sqlite` driver (`sqlite`) is mapped to `QUESTION` and used by
  the test suite, so the tests need no cgo. `mattn/go-sqlite3` is no longer a dependency.
- Integrated upstream fixes: a `?` inside a string or comment is left alone by `Rebind`
  (#768), `Conn` has the named-query methods of `DB`/`Tx` (#780), a PostgreSQL `::` cast
  right after a named parameter is supported (#1004), `Get` no longer touches the
  destination when it returns `sql.ErrNoRows` (#1005), a password in a connection error is
  masked (#1006), plus #757, #798, #805, #811, #846, #858, #938, #953, #962, #979, #993
  and #1003.
- `any` instead of `interface{}`; Go 1.24 or later is required.
- CI runs on GitHub Actions only, the old CircleCI config is gone.

## Notes

- `Row.Scan` refuses `*sql.RawBytes`, like `database/sql` does for `QueryRow`.
- Multi-statement `Exec` behaves differently per driver: `pq` runs them all, sqlite does
  not.
- A `SELECT *` with nullable columns needs `sql.Null*` (or a pointer) in the destination
  struct.
- `sqlx.Open`/`Connect` mask the password of a malformed DSN in the returned error.

## Development

```shell
make test-race   # go test -race -count=1 ./...
make lint        # go vet ./... && staticcheck -checks=all ./...
make vuln-check  # govulncheck ./...
```

CI runs `go test ./...` and `go vet ./...` on Go 1.24.x and 1.26.x against a PostgreSQL
service. Pushing a `v*` tag runs the `Tag-release` workflow, which writes the release
notes from the commit log with [chlog](https://github.com/gookit/gitw).

## License

MIT, see [LICENSE](LICENSE).
