package pq

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/bmizerany/pq.go/proto"
	"io"
	"net"
	"net/url"
	"path"
	"strings"
)

type Driver struct{}

func (dr *Driver) Open(name string) (driver.Conn, error) {
	return OpenRaw(name)
}

func OpenRaw(uarel string) (*Conn, error) {
	u, err := url.Parse(uarel)
	if err != nil {
		return nil, err
	}

	if strings.Index(u.Host, ":") < 0 {
		u.Host += ":5432"
	}

	nc, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}

	params := make(proto.Values)
	params.Set("user", u.User.Username())
	if u.Path != "" {
		params.Set("database", path.Base(u.Path))
	}

	pw, _ := u.User.Password()
	return New(nc, params, pw)
}

var defaultDriver = &Driver{}

func init() {
	sql.Register("postgres", defaultDriver)
}

type Conn struct {
	Settings proto.Values
	Pid      int
	Secret   int
	Status   byte
	Notifies <-chan *proto.Notify
	User     string

	rwc io.ReadWriteCloser
	p   *proto.Conn
	err error
}

func New(rwc io.ReadWriteCloser, params proto.Values, pw string) (*Conn, error) {
	notifies := make(chan *proto.Notify, 5) // 5 should be enough to prevent simple blocking

	cn := &Conn{
		Notifies: notifies,
		Settings: make(proto.Values),
		User:     params.Get("user"),
		p:        proto.New(rwc, notifies),
	}

	err := cn.p.Startup(params)
	if err != nil {
		return nil, err
	}

	for {
		m, err := cn.p.Next()
		if err != nil {
			return nil, err
		}

		if m.Err != nil {
			return nil, m.Err
		}

		switch m.Type {
		default:
			notExpected(m.Type)
		case 'R':
			switch m.Auth {
			case proto.AuthOk:
				continue
			case proto.AuthPlain:
				err := cn.p.Password(pw)
				if err != nil {
					rwc.Close()
					return nil, err
				}
			case proto.AuthMd5:
				err := cn.p.PasswordMd5(m.Salt, cn.User, pw)
				if err != nil {
					rwc.Close()
					return nil, err
				}
			default:
				return nil, fmt.Errorf("pq: unknown authentication type (%d)", m.Auth)
			}
		case 'S':
			cn.Settings.Set(m.Key, m.Val)
		case 'K':
			cn.Pid = m.Pid
			cn.Secret = m.Secret
		case 'Z':
			return cn, nil
		}
	}

	panic("not reached")
}

func (cn *Conn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if len(args) == 0 {
		err := cn.p.SimpleQuery(query)
		if err != nil {
			return nil, err
		}

		var serr error
		for {
			m, err := cn.p.Next()
			if err != nil {
				return nil, err
			}

			switch m.Type {
			case 'E':
				serr = m.Err
			case 'Z':
				return driver.RowsAffected(0), serr
			}
		}
	} else {
		stmt, err := cn.Prepare(query)
		if err != nil {
			return nil, err
		}

		return stmt.Exec(args)
	}

	panic("not reached")
}

func (cn *Conn) Prepare(query string) (driver.Stmt, error) {
	name := "" //TODO: support named queries

	stmt := &Stmt{
		Name:  name,
		query: query,
		p:     cn.p,
	}

	err := stmt.Parse()
	if err != nil {
		return nil, err
	}

	err = stmt.Describe()
	if err != nil {
		return nil, err
	}

	return stmt, nil
}

type Stmt struct {
	Name string

	query  string
	p      *proto.Conn
	params []int
	names  []string
	err    error
}

func (stmt *Stmt) Parse() error {
	err := stmt.p.Parse(stmt.Name, stmt.query)
	if err != nil {
		return err
	}

	err = stmt.p.Sync()
	if err != nil {
		return err
	}

	var serr error
	for {
		m, err := stmt.p.Next()
		if err != nil {
			return err
		}

		switch m.Type {
		default:
			notExpected(m.Type)
		case '1':
			// ignore
		case 'Z':
			return serr
		case 'E':
			serr = m.Err
		}
	}

	panic("not reached")
}

func (stmt *Stmt) Describe() error {
	err := stmt.p.Describe(proto.Statement, stmt.Name)
	if err != nil {
		return err
	}

	err = stmt.p.Sync()
	if err != nil {
		return err
	}

	var serr error
	for {
		m, err := stmt.p.Next()
		if err != nil {
			return err
		}

		switch m.Type {
		default:
			notExpected(m.Type)
		case 'E':
			serr = m.Err
		case 'n':
			// no data
		case 't':
			stmt.params = m.Params
		case 'T':
			stmt.names = m.ColNames
		case 'Z':
			return serr
		}
	}

	panic("not reached")
}

func (stmt *Stmt) Close() (err error) {
	err = stmt.p.ClosePP(proto.Statement, stmt.Name)
	if err != nil {
		return err
	}

	err = stmt.p.Sync()
	if err != nil {
		return err
	}

	var done bool
	for {
		m, err := stmt.p.Next()
		if err != nil {
			return err
		}
		if m.Err != nil {
			return m.Err
		}

		if m.Type == '3' {
			done = true
		}

		if done && m.Type == 'Z' {
			return nil
		}
	}

	panic("not reached")
}

func (stmt *Stmt) NumInput() int {
	return len(stmt.params)
}

func (stmt *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	// NOTE: should return []drive.Result, because a PS can have more
	// than one statement and recv more than one tag.
	rows, err := stmt.Query(args)
	if err != nil {
		return nil, err
	}

	for err = rows.Next(nil); err == nil; err = rows.Next(nil) {
	}

	if err != io.EOF {
		// We got an error, now we need to read the rest of the messages
		for rows.Next(nil) != io.EOF {
		}

		return nil, err
	}

	// TODO: use the tag given by CommandComplete
	return driver.RowsAffected(0), nil
}

func (stmt *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	// For now, we'll just say they're strings
	iargs := make([]interface{}, len(args))
	for i, a := range args {
		iargs[i] = a
	}
	sargs := encodeParams(iargs)

	err := stmt.p.Bind(stmt.Name, stmt.Name, sargs...)
	if err != nil {
		return nil, err
	}

	err = stmt.p.Execute(stmt.Name, 0)
	if err != nil {
		return nil, err
	}

	err = stmt.p.Sync()
	if err != nil {
		return nil, err
	}

	for {
		m, err := stmt.p.Next()
		if err != nil {
			return nil, err
		}
		if m.Err != nil {
			return nil, m.Err
		}

		switch m.Type {
		default:
			notExpected(m.Type)
		case '2':
			rows := &Rows{
				p:     stmt.p,
				names: stmt.names,
			}
			return rows, nil
		}
	}

	panic("not reached")
}

type Rows struct {
	p     *proto.Conn
	names []string
	c     int
	done  bool
}

func (r *Rows) Close() (err error) {
	// Drain the remaining rows
	for err == nil {
		err = r.Next(nil)
	}

	if err == io.EOF {
		return nil
	}

	return
}

func (r *Rows) Complete() int {
	return r.c
}

func (r *Rows) Columns() []string {
	return r.names
}

func (r *Rows) Next(dest []driver.Value) (err error) {
	if r.done {
		return io.EOF
	}

	var m *proto.Msg
	for {
		m, err = r.p.Next()
		if err != nil {
			return err
		}
		if m.Err != nil {
			return m.Err
		}

		switch m.Type {
		default:
			notExpected(m.Type)
		case 'D':
			for i := 0; i < len(dest); i++ {
				if m.Cols[i] == nil {
					dest[i] = nil
				} else {
					dest[i] = string(m.Cols[i])
				}
			}
			return nil
		case 'C':
			r.c++
		case 'Z':
			r.done = true
			return io.EOF
		}
	}

	panic("not reached")
}

func (cn *Conn) Begin() (driver.Tx, error) {
	_, err := cn.Exec("BEGIN", nil)
	if err != nil {
		return nil, err
	}

	return &Tx{cn}, nil
}

func (cn *Conn) Close() error {
	return cn.p.Close()
}

func notExpected(c byte) {
	panic(fmt.Sprintf("pq: unexpected response from server (%c)", c))
}

type Tx struct {
	cn *Conn
}

func (t *Tx) Commit() error {
	_, err := t.cn.Exec("COMMIT", nil)
	return err
}

func (t *Tx) Rollback() error {
	_, err := t.cn.Exec("ROLLBACK", nil)
	return err
}
