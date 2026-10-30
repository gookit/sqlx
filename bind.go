package sqlx

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gookit/sqlx/reflectx"
)

// Bindvar types supported by Rebind, BindMap and BindStruct.
const (
	UNKNOWN = iota
	QUESTION
	DOLLAR
	NAMED
	AT
)

var defaultBinds = map[int][]string{
	DOLLAR:   {"postgres", "pgx", "pgx/v4", "pgx/v5", "pq-timeouts", "cloudsqlpostgres", "ql", "nrpostgres", "cockroach"},
	QUESTION: {"mysql", "sqlite", "sqlite3", "nrmysql", "nrsqlite3"},
	NAMED:    {"oci8", "ora", "goracle", "godror"},
	AT:       {"sqlserver", "azuresql"},
}

var binds sync.Map

func init() {
	for bind, drivers := range defaultBinds {
		for _, driver := range drivers {
			BindDriver(driver, bind)
		}
	}

}

// BindType returns the bindtype for a given database given a drivername.
func BindType(driverName string) int {
	itype, ok := binds.Load(driverName)
	if !ok {
		return UNKNOWN
	}
	return itype.(int)
}

// BindDriver sets the BindType for driverName to bindType.
func BindDriver(driverName string, bindType int) {
	binds.Store(driverName, bindType)
}

// Rebind a query from the default bindtype (QUESTION) to the target bindtype.
//
// A '?' inside a string constant ('...' or "..."), a quoted identifier or a SQL comment
// (-- to the end of the line, /* ... */) is left alone, so `WHERE name = 'a?b'` and a
// commented out `?` do not consume a placeholder. A quote is escaped by doubling it
// ('it”s') or with a backslash ('it\'s'), as the drivers accept.
func Rebind(bindType int, query string) string {
	switch bindType {
	case QUESTION, UNKNOWN:
		return query
	}

	// room for a few params before the append has to grow
	rqb := make([]byte, 0, len(query)+10)

	var n int         // placeholder counter
	var quote byte    // active quote character, 0 outside a string
	var lineCmt bool  // inside a -- comment
	var blockCmt bool // inside a /* */ comment

	for i := 0; i < len(query); i++ {
		c := query[i]

		if lineCmt {
			rqb = append(rqb, c)
			if c == '\n' {
				lineCmt = false
			}
			continue
		}
		if blockCmt {
			rqb = append(rqb, c)
			if c == '*' && i+1 < len(query) && query[i+1] == '/' {
				rqb = append(rqb, '/')
				i++
				blockCmt = false
			}
			continue
		}

		if quote != 0 {
			rqb = append(rqb, c)
			if c == '\\' && i+1 < len(query) {
				// a backslash escapes the next byte inside the string
				rqb = append(rqb, query[i+1])
				i++
				continue
			}
			if c == quote {
				// a doubled quote is an escaped quote, not the end of the string
				if i+1 < len(query) && query[i+1] == quote {
					rqb = append(rqb, query[i+1])
					i++
					continue
				}
				quote = 0
			}
			continue
		}

		switch c {
		case '?':
			n++
			switch bindType {
			case DOLLAR:
				rqb = append(rqb, '$')
			case NAMED:
				rqb = append(rqb, ':', 'a', 'r', 'g')
			case AT:
				rqb = append(rqb, '@', 'p')
			}
			rqb = strconv.AppendInt(rqb, int64(n), 10)
		case '\'', '"':
			quote = c
			rqb = append(rqb, c)
		case '-':
			if i+1 < len(query) && query[i+1] == '-' {
				lineCmt = true
				rqb = append(rqb, '-', '-')
				i++
				continue
			}
			rqb = append(rqb, c)
		case '/':
			if i+1 < len(query) && query[i+1] == '*' {
				blockCmt = true
				rqb = append(rqb, '/', '*')
				i++
				continue
			}
			rqb = append(rqb, c)
		default:
			rqb = append(rqb, c)
		}
	}

	return string(rqb)
}

// Experimental implementation of Rebind which uses a bytes.Buffer.  The code is
// much simpler and should be more resistant to odd unicode, but it is twice as
// slow.  Kept here for benchmarking purposes and to possibly replace Rebind if
// problems arise with its somewhat naive handling of unicode.
func rebindBuff(bindType int, query string) string {
	if bindType != DOLLAR {
		return query
	}

	b := make([]byte, 0, len(query))
	rqb := bytes.NewBuffer(b)
	j := 1
	for _, r := range query {
		if r == '?' {
			rqb.WriteRune('$')
			rqb.WriteString(strconv.Itoa(j))
			j++
		} else {
			rqb.WriteRune(r)
		}
	}

	return rqb.String()
}

func asSliceForIn(i any) (v reflect.Value, ok bool) {
	if i == nil {
		return reflect.Value{}, false
	}

	v = reflect.ValueOf(i)
	t := reflectx.Deref(v.Type())

	// Only expand slices
	if t.Kind() != reflect.Slice {
		return reflect.Value{}, false
	}

	// []byte is a driver.Value type so it should not be expanded
	if t == reflect.TypeOf([]byte{}) {
		return reflect.Value{}, false

	}

	return v, true
}

// In expands slice values in args, returning the modified query string
// and a new arg list that can be executed by a database. The `query` should
// use the `?` bindVar.  The return value uses the `?` bindVar.
//
// In only expands `?` placeholders, whatever the driver: with PostgreSQL write
// `IN (?)` and then rebind the result with db.Rebind, instead of writing `IN ($1)`,
// which would leave the placeholder unexpanded and fail with a bindVar count error.
func In(query string, args ...any) (string, []any, error) {
	// argMeta stores reflect.Value and length for slices and
	// the value itself for non-slice arguments
	type argMeta struct {
		v      reflect.Value
		i      any
		length int
	}

	var flatArgsCount int
	var anySlices bool

	var stackMeta [32]argMeta

	var meta []argMeta
	if len(args) <= len(stackMeta) {
		meta = stackMeta[:len(args)]
	} else {
		meta = make([]argMeta, len(args))
	}

	for i, arg := range args {
		if a, ok := arg.(driver.Valuer); ok {
			var err error
			if arg, err = callValuerValue(a); err != nil {
				return "", nil, err
			}
			// keep the unwrapped value for the no-slice fast path below
			args[i] = arg
		}

		if v, ok := asSliceForIn(arg); ok {
			meta[i].length = v.Len()
			meta[i].v = v

			anySlices = true
			flatArgsCount += meta[i].length

			if meta[i].length == 0 {
				return "", nil, errors.New("empty slice passed to 'in' query")
			}
		} else {
			meta[i].i = arg
			flatArgsCount++
		}
	}

	// don't do any parsing if there aren't any slices;  note that this means
	// some errors that we might have caught below will not be returned.
	if !anySlices {
		return query, args, nil
	}

	newArgs := make([]any, 0, flatArgsCount)

	var buf strings.Builder
	buf.Grow(len(query) + len(", ?")*flatArgsCount)

	var arg, offset int

	for i := strings.IndexByte(query[offset:], '?'); i != -1; i = strings.IndexByte(query[offset:], '?') {
		if arg >= len(meta) {
			// if an argument wasn't passed, lets return an error;  this is
			// not actually how database/sql Exec/Query works, but since we are
			// creating an argument list programmatically, we want to be able
			// to catch these programmer errors earlier.
			return "", nil, errors.New("number of bindVars exceeds arguments")
		}

		argMeta := meta[arg]
		arg++

		// not a slice, continue.
		// our questionmark will either be written before the next expansion
		// of a slice or after the loop when writing the rest of the query
		if argMeta.length == 0 {
			offset = offset + i + 1
			newArgs = append(newArgs, argMeta.i)
			continue
		}

		// write everything up to and including our ? character
		buf.WriteString(query[:offset+i+1])

		for si := 1; si < argMeta.length; si++ {
			buf.WriteString(", ?")
		}

		newArgs = appendReflectSlice(newArgs, argMeta.v, argMeta.length)

		// slice the query and reset the offset. this avoids some bookkeeping for
		// the write after the loop
		query = query[offset+i+1:]
		offset = 0
	}

	buf.WriteString(query)

	if arg < len(meta) {
		return "", nil, errors.New("number of bindVars less than number arguments")
	}

	return buf.String(), newArgs, nil
}

func appendReflectSlice(args []any, v reflect.Value, vlen int) []any {
	switch val := v.Interface().(type) {
	case []any:
		args = append(args, val...)
	case []int:
		for i := range val {
			args = append(args, val[i])
		}
	case []string:
		for i := range val {
			args = append(args, val[i])
		}
	default:
		for si := 0; si < vlen; si++ {
			args = append(args, v.Index(si).Interface())
		}
	}

	return args
}

// callValuerValue returns vr.Value(), except when vr is a nil pointer whose element type
// implements driver.Valuer: calling Value on it would panic, so it is treated as nil.
// This mirrors the unexported helper of the database/sql package, so `In` accepts a nil
// pointer to a value type the same way database/sql does.
func callValuerValue(vr driver.Valuer) (v driver.Value, err error) {
	if rv := reflect.ValueOf(vr); rv.Kind() == reflect.Pointer &&
		rv.IsNil() &&
		rv.Type().Elem().Implements(_valuerInterface) {
		return nil, nil
	}
	return vr.Value()
}
