package sqlx

import (
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	_ "modernc.org/sqlite"
)

type moderncPerson struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func TestModerncDialect(t *testing.T) {
	t.Run("bind type", func(t *testing.T) {
		assert.Equal(t, QUESTION, BindType("sqlite"))
		assert.Equal(t, DOLLAR, BindType("pgx"))
	})

	t.Run("rebind", func(t *testing.T) {
		query := "SELECT * FROM people WHERE id = ? AND name = ?"
		assert.Equal(t, query, Rebind(BindType("sqlite"), query))
		assert.Equal(t, "SELECT * FROM people WHERE id = $1 AND name = $2", Rebind(BindType("pgx"), query))
	})

	t.Run("named exec, select and get", func(t *testing.T) {
		db, err := Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
		assert.NoErr(t, err)
		if err != nil {
			return
		}
		defer db.Close()

		_, err = db.Exec(`CREATE TABLE people (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
		assert.NoErr(t, err)

		_, err = db.NamedExec("INSERT INTO people (id, name) VALUES (:id, :name)", moderncPerson{ID: 1, Name: "Ada"})
		assert.NoErr(t, err)
		_, err = db.NamedExec("INSERT INTO people (id, name) VALUES (:id, :name)", moderncPerson{ID: 2, Name: "Grace"})
		assert.NoErr(t, err)

		var people []moderncPerson
		err = db.Select(&people, "SELECT id, name FROM people WHERE id > ? ORDER BY id", 0)
		assert.NoErr(t, err)
		assert.Equal(t, []moderncPerson{{ID: 1, Name: "Ada"}, {ID: 2, Name: "Grace"}}, people)

		var person moderncPerson
		err = db.Get(&person, "SELECT id, name FROM people WHERE id = ?", 2)
		assert.NoErr(t, err)
		assert.Equal(t, moderncPerson{ID: 2, Name: "Grace"}, person)
	})
}
