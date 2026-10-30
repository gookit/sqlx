package sqlx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"
)

// fixes ported from the upstream pull requests listed in the comment of each test.

func newTestSQLite(t *testing.T) *DB {
	t.Helper()
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// #979: Tx.Stmtx returned a Stmt without the tx's unsafe flag.
func TestTxStmtxKeepsUnsafe(t *testing.T) {
	db := newTestSQLite(t).Unsafe()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Preparex("SELECT 1")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer func() { _ = stmt.Close() }()

	if !tx.Stmtx(stmt).unsafe {
		t.Error("Tx.Stmtx should carry the unsafe flag of the transaction")
	}
}

// #993: mapperFor/isUnsafe missed Conn and *Conn, so a Stmt built from a Conn fell back
// to the default mapper.
func TestConnKeepsMapperAndUnsafe(t *testing.T) {
	db := newTestSQLite(t).Unsafe()
	conn, err := db.Connx(context.Background())
	if err != nil {
		t.Fatalf("connx: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if mapperFor(conn) != db.Mapper {
		t.Error("mapperFor(*Conn) should return the Conn mapper")
	}
	if mapperFor(*conn) != db.Mapper {
		t.Error("mapperFor(Conn) should return the Conn mapper")
	}
	if !isUnsafe(conn) || !isUnsafe(*conn) {
		t.Error("isUnsafe should report the unsafe flag of a Conn")
	}
}

// #846: a Row checked only through Err leaked the underlying rows.
func TestRowErrClosesRows(t *testing.T) {
	db := newTestSQLite(t)
	row := db.QueryRowx("SELECT 1")
	if err := row.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if err := row.rows.Err(); err != nil {
		if err.Error() != "sql: Rows are closed" {
			t.Fatalf("rows should be closed after Err, got %v", err)
		}
	} else if row.rows.Next() {
		t.Error("rows should be closed after Err")
	}

	// a Row built from a binding error has no rows and must not panic
	if err := (&Row{err: sql.ErrNoRows}).Err(); err != sql.ErrNoRows {
		t.Errorf("expected the deferred error, got %v", err)
	}
}

type testValuer struct{ Val string }

func (v testValuer) Value() (driver.Value, error) { return v.Val, nil }

// #953: In panicked on a nil pointer whose type implements driver.Valuer.
func TestInNilValuerPointer(t *testing.T) {
	var p *testValuer

	q, args, err := In("SELECT * FROM t WHERE x = ? AND y = ?", p, "b")
	if err != nil {
		t.Fatalf("In: %v", err)
	}
	if q != "SELECT * FROM t WHERE x = ? AND y = ?" {
		t.Errorf("unexpected query %q", q)
	}
	if len(args) != 2 || args[0] != nil || args[1] != "b" {
		t.Errorf("expected [nil b], got %#v", args)
	}
}

// #757: NamedExec bound the driver.Valuer struct itself instead of the value.
func TestBindArgsUnwrapsValuer(t *testing.T) {
	type form struct {
		Name sql.NullString
	}
	args, err := bindAnyArgs([]string{"name"}, form{Name: sql.NullString{String: "va", Valid: true}}, mapper())
	if err != nil {
		t.Fatalf("bindArgs: %v", err)
	}
	if len(args) != 1 || args[0] != "va" {
		t.Errorf("expected [va], got %#v", args)
	}
}

// #805: a pointer to a slice could not be bound for batch insert ([]*T already worked).
func TestBindNamedPtrToSlice(t *testing.T) {
	type person struct {
		Name string
	}
	people := []*person{{Name: "a"}, {Name: "b"}}

	query, args, err := bindNamedMapper(QUESTION, "INSERT INTO t (name) VALUES (:name)", &people, mapper())
	if err != nil {
		t.Fatalf("bindNamedMapper: %v", err)
	}
	if len(args) != 2 || args[0] != "a" || args[1] != "b" {
		t.Errorf("expected [a b], got %#v", args)
	}
	want := "INSERT INTO t (name) VALUES (?),(?)"
	if query != want {
		t.Errorf("unexpected query %q, want %q", query, want)
	}
}

// #811: fixBound assumed the named placeholder list only came from an INSERT.
func TestFixBoundValuesInSelect(t *testing.T) {
	in := "INSERT INTO t (a) VALUES (?)"
	if got := fixBound(in, 3); got != "INSERT INTO t (a) VALUES (?),(?),(?)" {
		t.Errorf("insert: unexpected %q", got)
	}

	sel := "SELECT * FROM (VALUES (?)) AS tmp(a)"
	if got := fixBound(sel, 2); got != "SELECT * FROM (VALUES (?),(?)) AS tmp(a)" {
		t.Errorf("select: unexpected %q", got)
	}
}
