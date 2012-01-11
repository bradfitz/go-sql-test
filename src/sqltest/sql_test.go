package sqltest

import (
	"exp/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMisc(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	var blob = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "foo.db"))
	if err != nil {
		t.Fatalf("foo.db open fail: %v", err)
	}
	db.Exec("create table foo (id integer primary key, bar blob[16])")
	db.Exec("insert or replace into foo (id, bar) values(?,?)", 0, blob)
	b := make([]byte, 16)
	db.QueryRow("select bar from foo where id = ?", 0).Scan(&b)
	got := fmt.Sprintf("%x", b)
	want := fmt.Sprintf("%x", blob)
	if got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}
