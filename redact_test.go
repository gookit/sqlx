package sqlx

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRedactURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		// a normal URL is redacted by url.URL.Redacted
		{
			"postgres://user:secret@localhost:5432/db?sslmode=disable",
			"postgres://user:xxxxx@localhost:5432/db?sslmode=disable",
		},
		{"postgres://user:secret@localhost/db", "postgres://user:xxxxx@localhost/db"},
		// no password: nothing to do
		{"postgres://user@localhost/db", "postgres://user@localhost/db"},
		{"postgres://localhost/db", "postgres://localhost/db"},
		// a space in the password makes url.Parse fail, the fallback still masks it
		{"postgres://user:se cret@localhost/db", "postgres://user:xxxxx@localhost/db"},
		// MySQL style DSN without a scheme
		{"user:secret@tcp(127.0.0.1:3306)/db", "user:xxxxx@tcp(127.0.0.1:3306)/db"},
		{"user:secret@tcp(127.0.0.1:3306)/db?parseTime=true", "user:xxxxx@tcp(127.0.0.1:3306)/db?parseTime=true"},
		// key-value DSN
		{"user=root password=secret dbname=app", "user=root password=xxxxx dbname=app"},
		{"pwd='a b c' host=localhost", "pwd=xxxxx host=localhost"},
		// plain text is left alone
		{"dial tcp 127.0.0.1:1: connect: connection refused", "dial tcp 127.0.0.1:1: connect: connection refused"},
	}

	for _, c := range cases {
		if got := RedactURL(c.in); got != c.want {
			t.Errorf("RedactURL(%q)\n got: %q\nwant: %q", c.in, got, c.want)
		}
	}
}

func TestRedactError(t *testing.T) {
	sentinel := errors.New("connection refused")

	// no credential in the message: the original error is returned as is
	plain := fmt.Errorf("dial failed: %w", sentinel)
	if got := RedactError(plain); got != plain {
		t.Errorf("RedactError should return the original error, got %v", got)
	}
	if RedactError(nil) != nil {
		t.Error("RedactError(nil) should be nil")
	}

	// a URL in the message is masked and the error still unwraps
	err := fmt.Errorf("cannot connect to postgres://user:secret@localhost/db: %w", sentinel)
	redacted := RedactError(err)
	if strings.Contains(redacted.Error(), "secret") {
		t.Errorf("password leaked: %v", redacted)
	}
	if !strings.Contains(redacted.Error(), "xxxxx") {
		t.Errorf("expected a masked password, got %v", redacted)
	}
	if !errors.Is(redacted, sentinel) {
		t.Error("the redacted error should still unwrap to the original")
	}
}

// echoDSNDriver fails every Open with the DSN it was given, the way a driver that cannot
// parse a malformed DSN does.
type echoDSNDriver struct{}

func (echoDSNDriver) Open(name string) (driver.Conn, error) {
	return nil, fmt.Errorf("cannot open %s", name)
}

func init() { sql.Register("sqlx_redact_test", echoDSNDriver{}) }

// Connect redacts the credentials a driver echoes back in its error.
func TestConnectRedactsDSN(t *testing.T) {
	const dsn = "appuser:s3cr3t@tcp(127.0.0.1:3306)/db"

	_, err := Connect("sqlx_redact_test", dsn)
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("password leaked: %v", err)
	}
	if !strings.Contains(err.Error(), "xxxxx") {
		t.Errorf("expected a masked password, got %v", err)
	}
}

// ConnectContext redacts the same way.
func TestConnectContextRedactsDSN(t *testing.T) {
	const dsn = "postgres://appuser:s3cr3t@127.0.0.1:1/db"

	_, err := ConnectContext(t.Context(), "sqlx_redact_test", dsn)
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("password leaked: %v", err)
	}
}
