package pq

import (
	"exp/sql"
	"fmt"
	"github.com/bmizerany/assert"
	"os"
	"testing"
)

var cs = fmt.Sprintf("postgres://%s:@localhost:5432", os.Getenv("USER"))

func TestSqlSimple(t *testing.T) {
	cn, err := sql.Open("postgres", cs)
	assert.Equalf(t, nil, err, "%v", err)

	rows, err := cn.Query("SELECT length($1) AS foo", "testing")
	assert.Equalf(t, nil, err, "%v", err)

	ok := rows.Next()
	assert.T(t, ok)

	var length int
	err = rows.Scan(&length)
	assert.Equalf(t, nil, err, "%v", err)

	assert.Equal(t, 7, length)
}
