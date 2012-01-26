package sqltest

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type Tester interface {
	RunTest(*testing.T, func(params))
}

var (
	mysql  Tester = &mysqlDB{}
	sqlite Tester = sqliteDB{}
)

type sqliteDB struct{}

type mysqlDB struct {
	once    sync.Once // guards init of running
	running bool      // whether port 3306 is listening
}

func (m *mysqlDB) Running() bool {
	m.once.Do(func() {
		c, err := net.Dial("tcp", "localhost:3306")
		if err == nil {
			m.running = true
			c.Close()
		}
	})
	return m.running
}

type params struct {
	dbType Tester
	*testing.T
	*sql.DB
}

func (t params) mustExec(sql string, args ...interface{}) sql.Result {
	res, err := t.DB.Exec(sql, args...)
	if err != nil {
		t.Fatalf("Error running %q: %v", sql, err)
	}
	return res
}

func (sqliteDB) RunTest(t *testing.T, fn func(params)) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "foo.db"))
	if err != nil {
		t.Fatalf("foo.db open fail: %v", err)
	}
	fn(params{sqlite, t, db})
}

func (mdb *mysqlDB) RunTest(t *testing.T, fn func(params)) {
	if !mdb.Running() {
		t.Logf("skipping test; no MySQL running on localhost:3306")
		return
	}
	user := os.Getenv("GOSQLTEST_MYSQL_USER")
	if user == "" {
		user = "root"
	}
	pass, err := os.Getenverror("GOSQLTEST_MYSQL_PASS")
	if err != nil {
		pass = "root"
	}
	dbName := "gosqltest"
	db, err := sql.Open("mymysql", fmt.Sprintf("%s/%s/%s", dbName, user, pass))
	if err != nil {
		t.Fatalf("error connecting: %v", err)
	}

	params := params{mysql, t, db}

	// Drop all tables in the test database.
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		t.Fatalf("failed to enumerate tables: %v", err)
	}
	for rows.Next() {
		var table string
		if rows.Scan(&table) == nil {
			params.mustExec("DROP TABLE " + table)
		}
	}

	fn(params)
}

func sqlBlobParam(t params, size int) string {
	if t.dbType == sqlite {
		return fmt.Sprintf("blob[%d]", size)
	}
	return fmt.Sprintf("VARBINARY(%d)", size)
}

func TestBlobs_SQLite(t *testing.T) { sqlite.RunTest(t, testBlobs) }
func TestBlobs_MySQL(t *testing.T)  { mysql.RunTest(t, testBlobs) }

func testBlobs(t params) {
	var blob = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	t.mustExec("create table foo (id integer primary key, bar " + sqlBlobParam(t, 16) + ")")
	t.mustExec("replace into foo (id, bar) values(?,?)", 0, blob)

	want := fmt.Sprintf("%x", blob)

	b := make([]byte, 16)
	err := t.QueryRow("select bar from foo where id = ?", 0).Scan(&b)
	got := fmt.Sprintf("%x", b)
	if err != nil {
		t.Errorf("[]byte scan: %v", err)
	} else if got != want {
		t.Errorf("for []byte, got %q; want %q", got, want)
	}

	err = t.QueryRow("select bar from foo where id = ?", 0).Scan(&got)
	want = string(blob)
	if err != nil {
		t.Errorf("string scan: %v", err)
	} else if got != want {
		t.Errorf("for string, got %q; want %q", got, want)
	}
}

func TestManyQueryRow_SQLite(t *testing.T) { sqlite.RunTest(t, testManyQueryRow) }
func TestManyQueryRow_MySQL(t *testing.T)  { mysql.RunTest(t, testManyQueryRow) }

func testManyQueryRow(t params) {
	t.mustExec("create table foo (id integer primary key, name varchar(50))")
	t.mustExec("insert into foo (id, name) values(?,?)", 1, "bob")
	var name string
	for i := 0; i < 10000; i++ {
		err := t.QueryRow("select name from foo where id = ?", 1).Scan(&name)
		if err != nil || name != "bob" {
			t.Fatalf("on query %d: err=%v, name=%q", i, err, name)
		}
	}
}


func TestTxQuery_SQLite(t *testing.T) { sqlite.RunTest(t, testTxQuery) }

func testTxQuery(t params) {
	tx, err := t.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("create table foo (id integer primary key, name varchar(50))")
	if err != nil {
		t.Fatal(err)
	}

	_, err = tx.Exec("insert into foo (id, name) values(?,?)", 1, "bob")
	if err != nil {
		t.Fatal(err)
	}

	r, err := tx.Query("select name from foo where id = ?", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if !r.Next() {
		if r.Err() != nil {
			t.Fatal(err)
		}
		t.Fatal("expected one rows")
	}

	var name string
	err = r.Scan(&name)
	if err != nil {
		t.Fatal(err)
	}
}
