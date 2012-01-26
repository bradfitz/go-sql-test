package pq

import (
	"github.com/bmizerany/assert"
	"io"
	"net"
	"os"
	"testing"
)

func TestConnPrepareErr(t *testing.T) {
	nc, err := net.Dial("tcp", "localhost:5432")
	assert.Equalf(t, nil, err, "%v", err)

	cn, err := New(nc, map[string]string{"user": os.Getenv("USER")}, "")
	assert.Equalf(t, nil, err, "%v", err)

	_, err = cn.Prepare("SELECT length($1) AS ZOMG! AN ERR")
	assert.NotEqual(t, nil, err)
}

func TestConnPrepare(t *testing.T) {
	nc, err := net.Dial("tcp", "localhost:5432")
	assert.Equalf(t, nil, err, "%v", err)

	cn, err := New(nc, map[string]string{"user": os.Getenv("USER")}, "")
	assert.Equalf(t, nil, err, "%v", err)

	stmt, err := cn.Prepare("SELECT length($1) AS foo WHERE true = $2")
	assert.Equalf(t, nil, err, "%v", err)
	assert.Equal(t, 2, stmt.NumInput())

	rows, err := stmt.Query([]interface{}{"testing", true})
	assert.Equalf(t, nil, err, "%v", err)
	assert.Equal(t, []string{"foo"}, rows.Columns())

	dest := make([]interface{}, 1)
	err = rows.Next(dest)
	assert.Equalf(t, nil, err, "%v", err)
	assert.Equal(t, []interface{}{"7"}, dest)

	err = rows.Next(dest)
	assert.Equalf(t, io.EOF, err, "%v", err)

	rows, err = stmt.Query([]interface{}{"testing", false})
	assert.Equalf(t, nil, err, "%v", err)
	assert.Equal(t, []string{"foo"}, rows.Columns())

	err = rows.Next(dest)
	assert.Equalf(t, io.EOF, err, "%v", err)
}

func TestConnNotify(t *testing.T) {
	nc, err := net.Dial("tcp", "localhost:5432")
	assert.Equalf(t, nil, err, "%v", err)

	cn, err := New(nc, map[string]string{"user": os.Getenv("USER")}, "")
	assert.Equalf(t, nil, err, "%v", err)

	// Listen
	lstmt, err := cn.Prepare("LISTEN test")
	assert.Equalf(t, nil, err, "%v", err)

	_, err = lstmt.Exec(nil)
	assert.Equalf(t, nil, err, "%v", err)

	err = lstmt.Close()
	assert.Equalf(t, nil, err, "%v", err)

	// Notify
	nstmt, err := cn.Prepare("NOTIFY test, 'foo'")
	assert.Equalf(t, nil, err, "%v", err)

	_, err = nstmt.Exec(nil)
	assert.Equalf(t, nil, err, "%v", err)

	err = nstmt.Close()
	assert.Equalf(t, nil, err, "%v", err)

	n := <-cn.Notifies
	assert.NotEqual(t, 0, n.Pid)
	assert.Equal(t, "test", n.From)
	assert.Equal(t, "foo", n.Payload)
}

func TestAuthCleartextPassword(t *testing.T) {
	c, err := OpenRaw("postgres://gopqtest:foo@localhost:5432/" + os.Getenv("USER"))
	//c, err := OpenRaw("postgres://gopqtest@localhost:5432/"+os.Getenv("USER"))
	assert.Equalf(t, nil, err, "%v", err)

	s, err := c.Prepare("SELECT 1")
	assert.Equalf(t, nil, err, "%v", err)

	_, err = s.Exec(nil)
	assert.Equalf(t, nil, err, "%v", err)
}

func TestAuthMissingCleartextPassword(t *testing.T) {
	c, err := OpenRaw("postgres://gopqtest@localhost:5432/" + os.Getenv("USER"))
	assert.NotEqual(t, nil, err)
	assert.Equal(t, (*Conn)(nil), c)
}

func TestAuthWrongCleartextPassword(t *testing.T) {
	c, err := OpenRaw("postgres://gopqtest:bar@localhost:5432/" + os.Getenv("USER"))
	assert.NotEqual(t, nil, err)
	assert.Equal(t, (*Conn)(nil), c)
}
