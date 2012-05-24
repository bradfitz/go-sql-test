package sqltest

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"testing"
)

type Tester interface {
	RunTest(*testing.T, func(params))
}

var (
	myMysql Tester = &myMysqlDB{}
	goMysql Tester = &goMysqlDB{}
	sqlite  Tester = sqliteDB{}
	pq      Tester = &pqDB{}
)

// pqDB validates the postgres driver by Blake Mizerany (github.com/bmizerany/pq.go)
type pqDB struct {
	once    sync.Once // guards init of running
	running bool      // whether port 5432 is listening
}

func (p *pqDB) RunTest(t *testing.T, fn func(params)) {
	if !p.Running() {
		fmt.Printf("skipping test; no Postgres running on localhost:5432\n")
		return
	}
	user := os.Getenv("GOSQLTEST_PQ_USER")
	if user == "" {
		user = os.Getenv("USER")
	}
	dbName := "gosqltest"
	db, err := sql.Open("postgres", fmt.Sprintf("user=%s password=foo dbname=%s sslmode=disable", user, dbName))
	if err != nil {
		t.Fatalf("error connecting: %v", err)
	}

	params := params{pq, t, db}

	// Drop all tables in the test database.
	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
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

func (p *pqDB) Running() bool {
	p.once.Do(func() {
		c, err := net.Dial("tcp", "localhost:5432")
		if err == nil {
			p.running = true
			c.Close()
		}
	})
	return p.running
}

type sqliteDB struct{}

type myMysqlDB struct {
	once    sync.Once // guards init of running
	running bool      // whether port 3306 is listening
}

func (m *myMysqlDB) Running() bool {
	m.once.Do(func() {
		c, err := net.Dial("tcp", "localhost:3306")
		if err == nil {
			m.running = true
			c.Close()
		}
	})
	return m.running
}

type goMysqlDB struct {
	once    sync.Once // guards init of running
	running bool      // whether port 3306 is listening
}

func (m *goMysqlDB) Running() bool {
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

var qrx = regexp.MustCompile(`\?`)

// q converts "?" characters to $1, $2, $n on postgres.
func (t params) q(sql string) string {
	if t.dbType != pq {
		return sql
	}
	n := 0
	return qrx.ReplaceAllStringFunc(sql, func(string) string {
		n++
		return "$" + strconv.Itoa(n)
	})
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

func (m *myMysqlDB) RunTest(t *testing.T, fn func(params)) {
	if !m.Running() {
		t.Logf("skipping test; no MySQL running on localhost:3306")
		return
	}
	user := os.Getenv("GOSQLTEST_MYSQL_USER")
	if user == "" {
		user = "root"
	}
	pass, ok := getenvOk("GOSQLTEST_MYSQL_PASS")
	if !ok {
		pass = "root"
	}
	dbName := "gosqltest"
	db, err := sql.Open("mymysql", fmt.Sprintf("%s/%s/%s", dbName, user, pass))
	if err != nil {
		t.Fatalf("error connecting: %v", err)
	}

	params := params{myMysql, t, db}

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

func (m *goMysqlDB) RunTest(t *testing.T, fn func(params)) {
	if !m.Running() {
		t.Logf("skipping test; no MySQL running on localhost:3306")
		return
	}
	user := os.Getenv("GOSQLTEST_MYSQL_USER")
	if user == "" {
		user = "root"
	}
	pass, ok := getenvOk("GOSQLTEST_MYSQL_PASS")
	if !ok {
		pass = "root"
	}
	dbName := "gosqltest"
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", user, pass, dbName))
	if err != nil {
		t.Fatalf("error connecting: %v", err)
	}

	params := params{goMysql, t, db}

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
	if t.dbType == pq {
		return "bytea"
	}
	return fmt.Sprintf("VARBINARY(%d)", size)
}

func TestBlobs_SQLite(t *testing.T)  { sqlite.RunTest(t, testBlobs) }
func TestBlobs_MyMySQL(t *testing.T) { myMysql.RunTest(t, testBlobs) }
func TestBlobs_GoMySQL(t *testing.T) { goMysql.RunTest(t, testBlobs) }
func TestBlobs_PQ(t *testing.T)      { pq.RunTest(t, testBlobs) }

func testBlobs(t params) {
	var blob = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	t.mustExec("create table foo (id integer primary key, bar " + sqlBlobParam(t, 16) + ")")
	t.mustExec(t.q("insert into foo (id, bar) values(?,?)"), 0, blob)

	want := fmt.Sprintf("%x", blob)

	b := make([]byte, 16)
	err := t.QueryRow(t.q("select bar from foo where id = ?"), 0).Scan(&b)
	got := fmt.Sprintf("%x", b)
	if err != nil {
		t.Errorf("[]byte scan: %v", err)
	} else if got != want {
		t.Errorf("for []byte, got %q; want %q", got, want)
	}

	err = t.QueryRow(t.q("select bar from foo where id = ?"), 0).Scan(&got)
	want = string(blob)
	if err != nil {
		t.Errorf("string scan: %v", err)
	} else if got != want {
		t.Errorf("for string, got %q; want %q", got, want)
	}
}

func TestManyQueryRow_SQLite(t *testing.T)  { sqlite.RunTest(t, testManyQueryRow) }
func TestManyQueryRow_MyMySQL(t *testing.T) { myMysql.RunTest(t, testManyQueryRow) }
func TestManyQueryRow_GoMySQL(t *testing.T) { goMysql.RunTest(t, testManyQueryRow) }
func TestManyQueryRow_PQ(t *testing.T)      { pq.RunTest(t, testManyQueryRow) }

func testManyQueryRow(t params) {
	if testing.Short() {
		t.Logf("skipping in short mode")
		return
	}
	t.mustExec("create table foo (id integer primary key, name varchar(50))")
	t.mustExec(t.q("insert into foo (id, name) values(?,?)"), 1, "bob")
	var name string
	for i := 0; i < 10000; i++ {
		err := t.QueryRow(t.q("select name from foo where id = ?"), 1).Scan(&name)
		if err != nil || name != "bob" {
			t.Fatalf("on query %d: err=%v, name=%q", i, err, name)
		}
	}
}

func TestTxQuery_SQLite(t *testing.T)  { sqlite.RunTest(t, testTxQuery) }
func TestTxQuery_MyMySQL(t *testing.T) { myMysql.RunTest(t, testTxQuery) }
func TestTxQuery_GoMySQL(t *testing.T) { goMysql.RunTest(t, testTxQuery) }
func TestTxQuery_PQ(t *testing.T)      { pq.RunTest(t, testTxQuery) }

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

	_, err = tx.Exec(t.q("insert into foo (id, name) values(?,?)"), 1, "bob")
	if err != nil {
		t.Fatal(err)
	}

	r, err := tx.Query(t.q("select name from foo where id = ?"), 1)
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

func TestPreparedStmt_SQLite(t *testing.T)  { sqlite.RunTest(t, testPreparedStmt) }
func TestPreparedStmt_MyMySQL(t *testing.T) { myMysql.RunTest(t, testPreparedStmt) }
func TestPreparedStmt_GoMySQL(t *testing.T) { goMysql.RunTest(t, testPreparedStmt) }
func TestPreparedStmt_PQ(t *testing.T)      { pq.RunTest(t, testPreparedStmt) }

func testPreparedStmt(t params) {
	t.mustExec("CREATE TABLE t (count INT)")
	sel, err := t.Prepare("SELECT count FROM t ORDER BY count DESC")
	if err != nil {
		t.Fatalf("prepare 1: %v", err)
	}
	ins, err := t.Prepare(t.q("INSERT INTO t (count) VALUES (?)"))
	if err != nil {
		t.Fatalf("prepare 2: %v", err)
	}

	for n := 1; n <= 3; n++ {
		if _, err := ins.Exec(n); err != nil {
			t.Fatalf("insert(%d) = %v", n, err)
		}
	}

	const nRuns = 10
	ch := make(chan bool)
	for i := 0; i < nRuns; i++ {
		go func() {
			defer func() {
				ch <- true
			}()
			for j := 0; j < 10; j++ {
				count := 0
				if err := sel.QueryRow().Scan(&count); err != nil && err != sql.ErrNoRows {
					t.Errorf("Query: %v", err)
					return
				}
				if _, err := ins.Exec(rand.Intn(100)); err != nil {
					t.Errorf("Insert: %v", err)
					return
				}
			}
		}()
	}
	for i := 0; i < nRuns; i++ {
		<-ch
	}
}

func getenvOk(k string) (v string, ok bool) {
	v = os.Getenv(k)
	if v != "" {
		return v, true
	}
	keq := k + "="
	for _, kv := range os.Environ() {
		if kv == keq {
			return "", true
		}
	}
	return "", false
}
