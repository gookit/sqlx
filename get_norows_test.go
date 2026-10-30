package sqlx

import (
	"database/sql"
	"testing"
)

// #1005: Get allocated the pointer fields of the destination before it knew whether the
// query returned a row, so a missing row still modified the destination.
func TestGetErrNoRowsKeepsDestination(t *testing.T) {
	db := newTestSQLite(t)
	if _, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t (id, name) VALUES (1, 'a')"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var d struct {
		ID   int
		Name *string
	}
	err := db.Get(&d, "SELECT id, name FROM t WHERE id = ?", 2)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
	if d.Name != nil {
		t.Errorf("Get modified the destination on ErrNoRows: Name=%v", *d.Name)
	}
	if d.ID != 0 {
		t.Errorf("Get modified the destination on ErrNoRows: ID=%d", d.ID)
	}

	// a matching row still fills the destination
	if err = db.Get(&d, "SELECT id, name FROM t WHERE id = ?", 1); err != nil {
		t.Fatalf("expected a row, got %v", err)
	}
	if d.Name == nil || *d.Name != "a" {
		t.Errorf("expected Name=a, got %#v", d)
	}
}

// StructScan on a Row keeps its behaviour: no rows is ErrNoRows, the destination untouched.
func TestStructScanErrNoRowsKeepsDestination(t *testing.T) {
	db := newTestSQLite(t)
	if _, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}

	var d struct {
		ID   int
		Name *string
	}
	err := db.QueryRowx("SELECT id, name FROM t WHERE id = ?", 1).StructScan(&d)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
	if d.Name != nil || d.ID != 0 {
		t.Errorf("StructScan modified the destination on ErrNoRows: %#v", d)
	}
}
