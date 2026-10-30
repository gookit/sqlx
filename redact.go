package sqlx

import (
	"net/url"
	"regexp"
)

// redactedPassword replaces a password in a redacted connection string.
const redactedPassword = "xxxxx"

var (
	// Userinfo of a URL whose password must be masked, for the strings url.Parse rejects
	// (a space or an unescaped '%' in the password, which is exactly when a driver echoes
	// the DSN back). The password stops at '/', so a query string is not swallowed.
	urlUserInfoRe = regexp.MustCompile(`(://[^:/@\s]+:)[^/@\r\n]*(@)`)
	// A MySQL style DSN at the start of a message or after a blank, quote or bracket:
	// user:password@protocol(address)/dbname.
	dsnUserInfoRe = regexp.MustCompile(`(^|[\s"'(\[])([^:@/\s]+:)[^@/\s]+(@)`)
	// password=... and pwd=..., the key-value form of a DSN.
	kvPasswordRe = regexp.MustCompile(`(?i)\b(password|pwd)\s*=\s*('[^']*'|"[^"]*"|[^\s;&]*)`)
)

// RedactURL masks the password of a database URL or DSN, so the string can be written to a
// log or returned in an error. A URL that net/url accepts is redacted by url.URL.Redacted;
// a malformed one falls back to masking the userinfo and any password=/pwd= value.
func RedactURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	out := rawURL
	if u, err := url.Parse(rawURL); err == nil && u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			out = u.Redacted()
		}
	}
	return redactMessage(out)
}

// redactMessage masks every credential it recognises in s.
func redactMessage(s string) string {
	if s == "" {
		return ""
	}
	s = urlUserInfoRe.ReplaceAllString(s, "${1}"+redactedPassword+"${2}")
	s = dsnUserInfoRe.ReplaceAllString(s, "${1}${2}"+redactedPassword+"${3}")
	s = kvPasswordRe.ReplaceAllString(s, "${1}=xxxxx")
	return s
}

// redactedError keeps the original error for errors.Is/As but reports a redacted message.
type redactedError struct {
	err error
	msg string
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.err }

// RedactError returns err unchanged when its message holds no credential. Otherwise it
// returns a wrapper that reports the redacted message and still unwraps to err, so
// errors.Is/errors.As keep working.
func RedactError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	redacted := redactMessage(msg)
	if redacted == msg {
		return err
	}
	return &redactedError{err: err, msg: redacted}
}

// redactError is the internal alias used on the connection paths.
func redactError(err error) error { return RedactError(err) }
