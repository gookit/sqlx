package sqlx

import (
	"context"
	"testing"
)

// #780: the Conn type was missing the named-query methods that DB and Tx have.
func TestConnNamedQueries(t *testing.T) {
	db := newTestSQLite(t)
	if _, err := db.Exec("CREATE TABLE person (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("create: %v", err)
	}

	ctx := context.Background()
	c, err := db.Connx(ctx)
	if err != nil {
		t.Fatalf("connx: %v", err)
	}
	defer func() { _ = c.Close() }()

	// binder: DriverName / Rebind / BindNamed
	if got := c.DriverName(); got != "sqlite" {
		t.Errorf("DriverName() = %q, want sqlite", got)
	}
	if got := c.Rebind("SELECT name FROM person WHERE name = ?"); got != "SELECT name FROM person WHERE name = ?" {
		t.Errorf("Rebind = %q", got)
	}
	q, args, err := c.BindNamed("SELECT name FROM person WHERE name = :name", map[string]any{"name": "a"})
	if err != nil {
		t.Fatalf("BindNamed: %v", err)
	}
	if q != "SELECT name FROM person WHERE name = ?" || len(args) != 1 || args[0] != "a" {
		t.Errorf("BindNamed = %q %#v", q, args)
	}

	// NamedExec / NamedQuery
	if _, err = c.NamedExec("INSERT INTO person (name) VALUES (:name)", map[string]any{"name": "a"}); err != nil {
		t.Fatalf("NamedExec: %v", err)
	}
	rows, err := c.NamedQuery("SELECT name FROM person WHERE name = :name", map[string]any{"name": "a"})
	if err != nil {
		t.Fatalf("NamedQuery: %v", err)
	}
	if !rows.Next() {
		t.Fatal("NamedQuery returned no row")
	}
	var name string
	if err = rows.Scan(&name); err != nil || name != "a" {
		t.Fatalf("scan: name=%q err=%v", name, err)
	}
	_ = rows.Close()

	// NamedExecContext / NamedQueryContext
	if _, err = c.NamedExecContext(ctx, "INSERT INTO person (name) VALUES (:name)", map[string]any{"name": "b"}); err != nil {
		t.Fatalf("NamedExecContext: %v", err)
	}
	nrows, err := c.NamedQueryContext(ctx, "SELECT name FROM person WHERE name = :name", map[string]any{"name": "b"})
	if err != nil {
		t.Fatalf("NamedQueryContext: %v", err)
	}
	if !nrows.Next() {
		t.Fatal("NamedQueryContext returned no row")
	}
	if err = nrows.Scan(&name); err != nil || name != "b" {
		t.Fatalf("scan: name=%q err=%v", name, err)
	}
	_ = nrows.Close()

	// Get / Select
	if err = c.Get(&name, "SELECT name FROM person WHERE name = ?", "a"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if name != "a" {
		t.Errorf("Get = %q, want a", name)
	}
	var names []string
	if err = c.Select(&names, "SELECT name FROM person ORDER BY name"); err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Errorf("Select = %#v", names)
	}

	// PrepareNamed
	ns, err := c.PrepareNamed("SELECT name FROM person WHERE name = :name")
	if err != nil {
		t.Fatalf("PrepareNamed: %v", err)
	}
	defer func() { _ = ns.Close() }()
	prows, err := ns.Queryx(map[string]any{"name": "a"})
	if err != nil {
		t.Fatalf("NamedStmt.Queryx: %v", err)
	}
	if !prows.Next() {
		t.Fatal("NamedStmt.Queryx returned no row")
	}
	_ = prows.Close()

	// Query / Queryx / QueryRowx / Exec / Prepare without a context
	r, err := c.Query("SELECT name FROM person WHERE name = ?", "a")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	_ = r.Close()

	xrows, err := c.Queryx("SELECT name FROM person WHERE name = ?", "a")
	if err != nil {
		t.Fatalf("Queryx: %v", err)
	}
	_ = xrows.Close()

	var got string
	if err = c.QueryRowx("SELECT name FROM person WHERE name = ?", "b").Scan(&got); err != nil || got != "b" {
		t.Fatalf("QueryRowx: name=%q err=%v", got, err)
	}
	if _, err = c.Exec("INSERT INTO person (name) VALUES (?)", "c"); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	st, err := c.Prepare("SELECT 1")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	_ = st.Close()

	// the Conn satisfies the interfaces the named helpers expect
	var _ Ext = c
	var _ ExtContext = c
}
