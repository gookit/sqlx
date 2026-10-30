package sqlx

import "testing"

// #1004: a PostgreSQL `::` cast immediately following a named parameter used to fail
// with "unexpected `:` while reading named param".
func TestCompileNamedQueryPostgresCast(t *testing.T) {
	cases := []struct {
		name  string
		query string
		q     string // QUESTION
		d     string // DOLLAR
		at    string // AT
		n     string // NAMED
		names []string
	}{
		{
			name:  "single cast",
			query: `SELECT :boundary::jsonb AS boundary`,
			q:     `SELECT ?::jsonb AS boundary`,
			d:     `SELECT $1::jsonb AS boundary`,
			at:    `SELECT @p1::jsonb AS boundary`,
			n:     `SELECT :boundary::jsonb AS boundary`,
			names: []string{"boundary"},
		},
		{
			name:  "two casts in the select list",
			query: `SELECT :boundary::jsonb, :id::text`,
			q:     `SELECT ?::jsonb, ?::text`,
			d:     `SELECT $1::jsonb, $2::text`,
			at:    `SELECT @p1::jsonb, @p2::text`,
			n:     `SELECT :boundary::jsonb, :id::text`,
			names: []string{"boundary", "id"},
		},
		{
			name:  "cast mixed with plain params",
			query: `SELECT * FROM person WHERE id=:id::int AND email=:email AND role=:role::varchar`,
			q:     `SELECT * FROM person WHERE id=?::int AND email=? AND role=?::varchar`,
			d:     `SELECT * FROM person WHERE id=$1::int AND email=$2 AND role=$3::varchar`,
			at:    `SELECT * FROM person WHERE id=@p1::int AND email=@p2 AND role=@p3::varchar`,
			n:     `SELECT * FROM person WHERE id=:id::int AND email=:email AND role=:role::varchar`,
			names: []string{"id", "email", "role"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, tc := range []struct {
				bindType int
				want     string
			}{
				{QUESTION, c.q}, {DOLLAR, c.d}, {AT, c.at}, {NAMED, c.n},
			} {
				got, names, err := compileNamedQuery([]byte(c.query), tc.bindType)
				if err != nil {
					t.Fatalf("bind type %d: unexpected error: %v", tc.bindType, err)
				}
				if got != tc.want {
					t.Errorf("bind type %d: got %q, want %q", tc.bindType, got, tc.want)
				}
				if len(names) != len(c.names) {
					t.Fatalf("bind type %d: got names %#v, want %#v", tc.bindType, names, c.names)
				}
				for i := range names {
					if names[i] != c.names[i] {
						t.Errorf("bind type %d: got names %#v, want %#v", tc.bindType, names, c.names)
						break
					}
				}
			}
		})
	}
}

// A cast that is not attached to a named parameter keeps both colons; it used to be
// collapsed to a single ':' ('x'::int came out as 'x':int, which is not valid SQL).
func TestCompileNamedQueryKeepsCastColons(t *testing.T) {
	cases := []struct{ in, want string }{
		{`SELECT '2026-05-01'::timestamptz AS starts_at`, `SELECT '2026-05-01'::timestamptz AS starts_at`},
		{`SELECT x::int`, `SELECT x::int`},
		{`SELECT "::foo" FROM a`, `SELECT "::foo" FROM a`},
	}
	for _, c := range cases {
		got, names, err := compileNamedQuery([]byte(c.in), QUESTION)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.in, got, c.want)
		}
		if len(names) != 0 {
			t.Errorf("%s: expected no names, got %#v", c.in, names)
		}
	}
}

// A single colon inside a parameter name is still an error.
func TestCompileNamedQuerySingleColonError(t *testing.T) {
	if _, _, err := compileNamedQuery([]byte("SELECT * FROM person WHERE id = :id:invalid"), QUESTION); err == nil {
		t.Fatal("expected an error for a single colon inside a named param")
	}
}

// End to end: Named() binds the parameter and keeps the cast.
func TestNamedPostgresCast(t *testing.T) {
	query := `SELECT :boundary::jsonb AS boundary, '2026-05-01'::timestamptz AS starts_at, :id::int AS id`
	args := map[string]any{
		"boundary": `{"type":"Polygon"}`,
		"id":       101,
	}

	q, gargs, err := Named(query, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `SELECT ?::jsonb AS boundary, '2026-05-01'::timestamptz AS starts_at, ?::int AS id`
	if q != want {
		t.Errorf("got query %q, want %q", q, want)
	}
	if len(gargs) != 2 || gargs[0] != `{"type":"Polygon"}` || gargs[1] != 101 {
		t.Errorf("unexpected bound args: %#v", gargs)
	}
}

// BindNamed with a struct and the PostgreSQL / MySQL bind types.
func TestBindNamedPostgresCast(t *testing.T) {
	type personArg struct {
		ID   int    `db:"id"`
		UUID string `db:"uuid"`
	}
	query := `SELECT * FROM person WHERE id = :id::int AND uuid = :uuid::uuid`
	arg := personArg{ID: 1, UUID: "abc-123"}

	for _, tc := range []struct {
		bindType int
		want     string
	}{
		{DOLLAR, `SELECT * FROM person WHERE id = $1::int AND uuid = $2::uuid`},
		{QUESTION, `SELECT * FROM person WHERE id = ?::int AND uuid = ?::uuid`},
	} {
		q, args, err := BindNamed(tc.bindType, query, arg)
		if err != nil {
			t.Fatalf("bind type %d: unexpected error: %v", tc.bindType, err)
		}
		if q != tc.want {
			t.Errorf("bind type %d: got %q, want %q", tc.bindType, q, tc.want)
		}
		if len(args) != 2 || args[0] != 1 || args[1] != "abc-123" {
			t.Errorf("bind type %d: unexpected args: %#v", tc.bindType, args)
		}
	}
}
