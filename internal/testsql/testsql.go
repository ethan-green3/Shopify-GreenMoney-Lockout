package testsql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

type Expectation struct {
	Kind          string
	QueryContains string
	Args          []any
	Columns       []string
	Rows          [][]driver.Value
	RowsAffected  int64
	Err           error
}

type State struct {
	mu           sync.Mutex
	expectations []Expectation
}

var (
	registerOnce sync.Once
	storeMu      sync.Mutex
	store        = map[string]*State{}
	counter      atomic.Int64
)

func Open(expectations []Expectation) (*sql.DB, *State, error) {
	registerOnce.Do(func() {
		sql.Register("testsql", driverImpl{})
	})

	id := fmt.Sprintf("testsql-%d", counter.Add(1))
	state := &State{expectations: append([]Expectation(nil), expectations...)}

	storeMu.Lock()
	store[id] = state
	storeMu.Unlock()

	db, err := sql.Open("testsql", id)
	if err != nil {
		return nil, nil, err
	}
	return db, state, nil
}

func (s *State) Verify() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.expectations) != 0 {
		return fmt.Errorf("unconsumed expectations: %d", len(s.expectations))
	}
	return nil
}

type driverImpl struct{}

func (driverImpl) Open(name string) (driver.Conn, error) {
	storeMu.Lock()
	state := store[name]
	storeMu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("unknown testsql state %q", name)
	}
	return &conn{state: state}, nil
}

type conn struct {
	state *State
}

func (c *conn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("not implemented") }
func (c *conn) Close() error                        { return nil }
func (c *conn) Begin() (driver.Tx, error)           { return tx{}, nil }
func (c *conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return tx{}, nil
}

func (c *conn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	exp, err := c.state.next("query", query, args)
	if err != nil {
		return nil, err
	}
	if exp.Err != nil {
		return nil, exp.Err
	}
	return &rows{columns: exp.Columns, rows: exp.Rows}, nil
}

func (c *conn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	exp, err := c.state.next("exec", query, args)
	if err != nil {
		return nil, err
	}
	if exp.Err != nil {
		return nil, exp.Err
	}
	return result(exp.RowsAffected), nil
}

func (c *conn) CheckNamedValue(*driver.NamedValue) error { return nil }

type rows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *rows) Columns() []string { return r.columns }
func (r *rows) Close() error      { return nil }

func (r *rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

type result int64

func (r result) LastInsertId() (int64, error) { return 0, fmt.Errorf("not supported") }
func (r result) RowsAffected() (int64, error) { return int64(r), nil }

type tx struct{}

func (tx) Commit() error   { return nil }
func (tx) Rollback() error { return nil }

func (s *State) next(kind, query string, args []driver.NamedValue) (Expectation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.expectations) == 0 {
		return Expectation{}, fmt.Errorf("unexpected %s query: %s", kind, query)
	}

	exp := s.expectations[0]
	s.expectations = s.expectations[1:]

	if exp.Kind != kind {
		return Expectation{}, fmt.Errorf("expected %s but got %s for query %s", exp.Kind, kind, query)
	}

	if normalize(exp.QueryContains) != "" && !strings.Contains(normalize(query), normalize(exp.QueryContains)) {
		return Expectation{}, fmt.Errorf("query %q does not contain %q", query, exp.QueryContains)
	}

	gotArgs := make([]any, 0, len(args))
	for _, arg := range args {
		gotArgs = append(gotArgs, arg.Value)
	}
	if exp.Args != nil && !reflect.DeepEqual(exp.Args, gotArgs) {
		return Expectation{}, fmt.Errorf("args mismatch: want %#v got %#v", exp.Args, gotArgs)
	}

	return exp, nil
}

func normalize(v string) string {
	return strings.Join(strings.Fields(v), " ")
}
