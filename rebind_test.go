package sqlx

import (
	"strings"
	"testing"
)

func TestRebindEscaped(t *testing.T) {
	cases := []struct {
		name  string
		query string
		d     string
		n     string
		at    string
	}{
		{
			name:  "plain params",
			query: `INSERT INTO foo (a, b, c) VALUES (?, ?, ?)`,
			d:     `INSERT INTO foo (a, b, c) VALUES ($1, $2, $3)`,
			n:     `INSERT INTO foo (a, b, c) VALUES (:arg1, :arg2, :arg3)`,
			at:    `INSERT INTO foo (a, b, c) VALUES (@p1, @p2, @p3)`,
		},
		{
			name:  "? inside a string constant",
			query: `SELECT * FROM t WHERE id = ? AND name = 'four?'`,
			d:     `SELECT * FROM t WHERE id = $1 AND name = 'four?'`,
			n:     `SELECT * FROM t WHERE id = :arg1 AND name = 'four?'`,
			at:    `SELECT * FROM t WHERE id = @p1 AND name = 'four?'`,
		},
		{
			name:  "? inside a quoted identifier",
			query: `SELECT "weird?col" FROM t WHERE id = ?`,
			d:     `SELECT "weird?col" FROM t WHERE id = $1`,
			n:     `SELECT "weird?col" FROM t WHERE id = :arg1`,
			at:    `SELECT "weird?col" FROM t WHERE id = @p1`,
		},
		{
			name:  "doubled quote inside a string",
			query: `SELECT 'it''s a ?' AS s, ? AS v`,
			d:     `SELECT 'it''s a ?' AS s, $1 AS v`,
			n:     `SELECT 'it''s a ?' AS s, :arg1 AS v`,
			at:    `SELECT 'it''s a ?' AS s, @p1 AS v`,
		},
		{
			name:  "backslash escaped quote",
			query: `SELECT 'it\'s a ?' AS s, ? AS v`,
			d:     `SELECT 'it\'s a ?' AS s, $1 AS v`,
			n:     `SELECT 'it\'s a ?' AS s, :arg1 AS v`,
			at:    `SELECT 'it\'s a ?' AS s, @p1 AS v`,
		},
		{
			name:  "line comment",
			query: "SELECT ? -- not a ? here\nFROM t WHERE id = ?",
			d:     "SELECT $1 -- not a ? here\nFROM t WHERE id = $2",
			n:     "SELECT :arg1 -- not a ? here\nFROM t WHERE id = :arg2",
			at:    "SELECT @p1 -- not a ? here\nFROM t WHERE id = @p2",
		},
		{
			name:  "block comment",
			query: `SELECT ? /* not a ? here */ FROM t WHERE id = ?`,
			d:     `SELECT $1 /* not a ? here */ FROM t WHERE id = $2`,
			n:     `SELECT :arg1 /* not a ? here */ FROM t WHERE id = :arg2`,
			at:    `SELECT @p1 /* not a ? here */ FROM t WHERE id = @p2`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// the QUESTION bind type is a no-op
			if got := Rebind(QUESTION, c.query); got != c.query {
				t.Errorf("QUESTION:\n got: %s\nwant: %s", got, c.query)
			}
			if got := Rebind(UNKNOWN, c.query); got != c.query {
				t.Errorf("UNKNOWN:\n got: %s\nwant: %s", got, c.query)
			}
			if got := Rebind(DOLLAR, c.query); got != c.d {
				t.Errorf("DOLLAR:\n got: %s\nwant: %s", got, c.d)
			}
			if got := Rebind(NAMED, c.query); got != c.n {
				t.Errorf("NAMED:\n got: %s\nwant: %s", got, c.n)
			}
			if got := Rebind(AT, c.query); got != c.at {
				t.Errorf("AT:\n got: %s\nwant: %s", got, c.at)
			}
		})
	}
}

// A query with many placeholders outgrows any small pre-sized buffer; the rewrite must
// still produce every parameter (an index-based buffer would panic here).
func TestRebindManyParams(t *testing.T) {
	query := "SELECT " + strings.Repeat("?, ", 200) + "?"

	got := Rebind(DOLLAR, query)
	if !strings.HasSuffix(got, "$201") {
		t.Errorf("expected the last param to be $201, got %s", got[max(0, len(got)-20):])
	}
	if n := strings.Count(got, "$"); n != 201 {
		t.Errorf("expected 201 placeholders, got %d", n)
	}

	got = Rebind(AT, query)
	if !strings.HasSuffix(got, "@p201") {
		t.Errorf("expected the last param to be @p201, got %s", got[max(0, len(got)-20):])
	}
}
