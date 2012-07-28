package libpq

/*
#include <stdlib.h>

static char **makeCharArray(int size) {
	return calloc(sizeof(char *), size);
}

static void setArrayString(char **a, char *s, int n) {
	a[n] = s;
}

static void freeArrayElements(int n, char **a) {
	int i;
	for (i = 0; i < n; i++) {
		free(a[i]);
		a[i] = NULL;
	}
}
*/
import "C"
import (
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"strconv"
	"time"
)

const timeFormat = "2006-01-02 15:04:05-07"

// wrapper for a request for a char** of length nargs
type pqPoolRequest struct {
	nargs int
	resp  chan (**C.char)
}

// return a char** of length nargs to the pool
type pqPoolReturn struct {
	nargs int
	array **C.char
}

var (
	argpool     map[int][]**C.char
	poolRequest chan pqPoolRequest
	poolReturn  chan pqPoolReturn
)

// convert database/sql/driver arguments into libpq-style char**
func buildCArgs(args []driver.Value) (**C.char, error) {
	carray := getCharArrayFromPool(len(args))

	for i, v := range args {
		var str string
		switch v := v.(type) {
		case int64:
			str = strconv.FormatInt(v, 10)
		case float64:
			str = strconv.FormatFloat(v, 'E', -1, 64)
		case bool:
			if v {
				str = "t"
			} else {
				str = "f"
			}
		case []byte:
			str = `\x` + hex.EncodeToString(v)
		case string:
			str = v
		case time.Time:
			str = v.Format(timeFormat)
		case nil:
			str = "NULL"
		default:
			returnCharArrayToPool(len(args), carray)
			return nil, errors.New("libpq: unsupported type")
		}

		C.setArrayString(carray, C.CString(str), C.int(i))
	}

	return carray, nil
}

func getCharArrayFromPool(nargs int) **C.char {
	ch := make(chan **C.char)
	req := pqPoolRequest{nargs, ch}
	poolRequest <- req
	return <-ch
}

func returnCharArrayToPool(nargs int, array **C.char) {
	C.freeArrayElements(C.int(nargs), array)
	poolReturn <- pqPoolReturn{nargs, array}
}

// this is started in a goroutine in libpq's init() to reduce the number
// of calls to makeCharArray()
func handleArgpool() {
	argpool = make(map[int][]**C.char)
	poolRequest = make(chan pqPoolRequest)
	poolReturn = make(chan pqPoolReturn)
	for {
		select {
		case req := <-poolReturn:
			list := append(argpool[req.nargs], req.array)
			argpool[req.nargs] = list

		case req := <-poolRequest:
			list := argpool[req.nargs]
			if len(list) == 0 {
				list = append(list, C.makeCharArray(C.int(req.nargs)))
			}
			req.resp <- list[0]
			argpool[req.nargs] = list[1:]
		}
	}
}
