package libpq

/*
#include <stdlib.h>
#include <libpq-fe.h>
*/
import "C"
import (
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	// Error returned by any call to LastInsertId().
	ErrLastInsertId = errors.New("libpq: LastInsertId() not supported")

	// Error returned by Open() if libpq is not thread-safe.
	ErrThreadSafety = errors.New("libpq: Not compiled for thread-safe operation")

	// Error returned by Open() if we could not determine Postgres OIDs.
	ErrFetchingOids = errors.New("libpq: Could not fetch base datatype OIDs")
)

type pqoid struct {
	Bytea       int
	Date        int
	Timestamp   int
	TimestampTz int
	Time        int
	TimeTz      int
}

type libpqDriver struct {
	sync.Mutex
	oids map[string]*pqoid
}

func init() {
	go handleArgpool()
	sql.Register("libpq", &libpqDriver{oids: make(map[string]*pqoid)})
}

// dsn is passed directly to PQconnectdb
func (d *libpqDriver) Open(dsn string) (driver.Conn, error) {
	if C.PQisthreadsafe() != 1 {
		return nil, ErrThreadSafety
	}

	params := C.CString(dsn)
	defer C.free(unsafe.Pointer(params))

	db := C.PQconnectdb(params)
	if C.PQstatus(db) != C.CONNECTION_OK {
		defer C.PQfinish(db)
		return nil, errors.New("libpq: connection error " + C.GoString(C.PQerrorMessage(db)))
	}

	oids, err := d.getOids(db, dsn)
	if err != nil {
		defer C.PQfinish(db)
		return nil, ErrFetchingOids
	}

	return &libpqConn{
		db:        db,
		oids:      oids,
		stmtCache: make(map[string]driver.Stmt),
		stmtNum:   0,
	}, nil
}

func (d *libpqDriver) getOids(db *C.PGconn, dsn string) (*pqoid, error) {
	var err error
	d.Lock()
	defer d.Unlock()

	// check cache
	if oids, ok := d.oids[dsn]; ok {
		return oids, nil
	}

	// not in cache - query the database
	oids := &pqoid{}
	names := []struct {
		kind string
		dest *int
	}{
		{"'bytea'", &oids.Bytea},
		{"'date'", &oids.Date},
		{"'timestamp'", &oids.Timestamp},
		{"'timestamp with time zone'", &oids.TimestampTz},
		{"'time'", &oids.Time},
		{"'time with time zone'", &oids.TimeTz},
	}

	// fetch all the OIDs we care about
	for _, n := range names {
		ccmd := C.CString("SELECT " + n.kind + "::regtype::oid")
		defer C.free(unsafe.Pointer(ccmd))
		cres := C.PQexec(db, ccmd)
		defer C.PQclear(cres)
		if err := resultError(cres); err != nil {
			return nil, err
		}
		sval := C.GoString(C.PQgetvalue(cres, 0, 0))
		*n.dest, err = strconv.Atoi(sval)
		if err != nil {
			return nil, ErrFetchingOids
		}
	}

	// save in cache for next time
	d.oids[dsn] = oids

	return oids, nil
}

type libpqConn struct {
	db        *C.PGconn
	oids      *pqoid
	stmtCache map[string]driver.Stmt
	stmtNum   int
}

func (c *libpqConn) Begin() (driver.Tx, error) {
	if _, err := c.exec("BEGIN", false); err != nil {
		return nil, err
	}
	return &libpqTx{c}, nil
}

type libpqTx struct {
	c *libpqConn
}

func (tx *libpqTx) Commit() error {
	_, err := tx.c.exec("COMMIT", false)
	return err
}

func (tx *libpqTx) Rollback() error {
	_, err := tx.c.exec("ROLLBACK", false)
	return err
}

func (c *libpqConn) Close() error {
	C.PQfinish(c.db)
	// free cached prepared statement names
	for _, v := range c.stmtCache {
		if stmt, ok := v.(*libpqStmt); ok {
			C.free(unsafe.Pointer(stmt.name))
		}
	}
	return nil
}

// Execute a query, possibly getting a result object (unless the caller doesn't
// want it, as in the case of BEGIN/COMMIT/ROLLBACK).
// the caller doesn't care about that (e.g., Begin(), Commit(), Rollback()).
//func (c *libpqConn) exec(cmd string, res *libpqResult) error {
func (c *libpqConn) exec(cmd string, wantResult bool) (driver.Result, error) {
	ccmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(ccmd))
	cres := C.PQexec(c.db, ccmd)
	defer C.PQclear(cres)
	if err := resultError(cres); err != nil {
		return nil, err
	}

	if !wantResult {
		return nil, nil
	}

	nrows, err := getNumRows(cres)
	if err != nil {
		return nil, err
	}

	return libpqResult(nrows), nil
}

// Execute a query with 1 or more parameters.
func (c *libpqConn) execParams(cmd string, args []driver.Value) (driver.Result, error) {
	// convert args into C array-of-strings
	cargs, err := buildCArgs(args)
	if err != nil {
		return nil, err
	}
	defer returnCharArrayToPool(len(args), cargs)

	ccmd := C.CString(cmd)
	defer C.free(unsafe.Pointer(ccmd))

	// execute
	cres := C.PQexecParams(c.db, ccmd, C.int(len(args)), nil, cargs, nil, nil, 0)
	defer C.PQclear(cres)
	if err = resultError(cres); err != nil {
		return nil, err
	}

	// get modified rows
	nrows, err := getNumRows(cres)
	if err != nil {
		return nil, err
	}

	return libpqResult(nrows), nil
}

// Implement Execer interface.
func (c *libpqConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if len(args) != 0 {
		return c.execParams(query, args)
	}

	return c.exec(query, true)
}

func (c *libpqConn) Prepare(query string) (driver.Stmt, error) {
	// check our connection's query cache to see if we've already prepared this
	cached, ok := c.stmtCache[query]
	if ok {
		return cached, nil
	}

	// create unique statement name
	// NOTE: do NOT free cname here because it is cached in c.stmtCache;
	//       all cached statement names are freed in c.Close()
	cname := C.CString(strconv.Itoa(c.stmtNum))
	c.stmtNum++
	cquery := C.CString(query)
	defer C.free(unsafe.Pointer(cquery))

	// initial query preparation
	cres := C.PQprepare(c.db, cname, cquery, 0, nil)
	defer C.PQclear(cres)
	if err := resultError(cres); err != nil {
		return nil, err
	}

	// get number of parameters in this query
	cinfo := C.PQdescribePrepared(c.db, cname)
	defer C.PQclear(cinfo)
	if err := resultError(cinfo); err != nil {
		return nil, err
	}
	nparams := int(C.PQnparams(cinfo))

	// save statement in cache
	stmt := &libpqStmt{c: c, name: cname, nparams: nparams}
	c.stmtCache[query] = stmt
	return stmt, nil
}

type libpqStmt struct {
	c       *libpqConn
	name    *C.char
	nparams int
}

func (s *libpqStmt) Close() error {
	// nothing to do - prepared statement names are cached and will be
	// freed in s.c.Close()
	return nil
}

func (s *libpqStmt) NumInput() int {
	return s.nparams
}

func (s *libpqStmt) exec(args []driver.Value) (*C.PGresult, error) {
	// convert args into C array-of-strings
	cargs, err := buildCArgs(args)
	if err != nil {
		return nil, err
	}
	defer returnCharArrayToPool(len(args), cargs)

	// execute
	cres := C.PQexecPrepared(s.c.db, s.name, C.int(len(args)), cargs, nil, nil, 0)
	if err = resultError(cres); err != nil {
		C.PQclear(cres)
		return nil, err
	}
	return cres, nil
}

func (s *libpqStmt) Exec(args []driver.Value) (driver.Result, error) {
	// execute prepared statement
	cres, err := s.exec(args)
	if err != nil {
		return nil, err
	}
	defer C.PQclear(cres)

	nrows, err := getNumRows(cres)
	if err != nil {
		return nil, err
	}

	return libpqResult(nrows), nil
}

func (s *libpqStmt) Query(args []driver.Value) (driver.Rows, error) {
	// execute prepared statement
	cres, err := s.exec(args)
	if err != nil {
		return nil, err
	}

	// check to see if this was a "LISTEN"
	if C.GoString(C.PQcmdStatus(cres)) == "LISTEN" {
		C.PQclear(cres)
		return &libpqListenRows{s.c}, nil
	}

	return &libpqRows{
		s:       s,
		res:     cres,
		ncols:   int(C.PQnfields(cres)),
		nrows:   int(C.PQntuples(cres)),
		currRow: 0,
		cols:    nil,
	}, nil
}

type libpqRows struct {
	s       *libpqStmt
	res     *C.PGresult
	ncols   int
	nrows   int
	currRow int
	cols    []string
}

func resultError(res *C.PGresult) error {
	status := C.PQresultStatus(res)
	if status == C.PGRES_COMMAND_OK || status == C.PGRES_TUPLES_OK {
		return nil
	}
	return errors.New("libpq: result error: " + C.GoString(C.PQresultErrorMessage(res)))
}

func getNumRows(cres *C.PGresult) (int64, error) {
	rowstr := C.GoString(C.PQcmdTuples(cres))
	if rowstr == "" {
		return 0, nil
	}

	return strconv.ParseInt(rowstr, 10, 64)
}

func (r *libpqRows) Close() error {
	C.PQclear(r.res)
	return nil
}

func (r *libpqRows) Columns() []string {
	if r.cols == nil {
		r.cols = make([]string, r.ncols)
		for i := 0; i < r.ncols; i++ {
			r.cols[i] = C.GoString(C.PQfname(r.res, C.int(i)))
		}
	}
	return r.cols
}

func (r *libpqRows) Next(dest []driver.Value) error {
	if r.currRow >= r.nrows {
		return io.EOF
	}
	currRow := C.int(r.currRow)
	r.currRow++

	for i := 0; i < len(dest); i++ {
		ci := C.int(i)

		// check for NULL
		if int(C.PQgetisnull(r.res, currRow, ci)) == 1 {
			dest[i] = nil
			continue
		}

		var err error
		val := C.GoString(C.PQgetvalue(r.res, currRow, ci))
		switch vtype := int(C.PQftype(r.res, ci)); vtype {
		case r.s.c.oids.Bytea:
			if !strings.HasPrefix(val, `\x`) {
				return errors.New("libpq: invalid byte string format")
			}
			dest[i], err = hex.DecodeString(val[2:])
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not decode hex string: %s", err))
			}
		case r.s.c.oids.Date:
			dest[i], err = time.Parse("2006-01-02", val)
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not parse DATE %s: %s", val, err))
			}
		case r.s.c.oids.Timestamp:
			dest[i], err = time.Parse("2006-01-02 15:04:05", val)
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not parse TIMESTAMP %s: %s", val, err))
			}
		case r.s.c.oids.TimestampTz:
			dest[i], err = time.Parse(timeFormat, val)
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not parse TIMESTAMP WITH TIME ZONE %s: %s", val, err))
			}
		case r.s.c.oids.Time:
			dest[i], err = time.Parse("15:04:05", val)
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not parse TIME %s: %s", val, err))
			}
		case r.s.c.oids.TimeTz:
			dest[i], err = time.Parse("15:04:05-07", val)
			if err != nil {
				return errors.New(fmt.Sprint("libpq: could not parse TIME WITH TIME ZONE %s: %s", val, err))
			}
		default:
			dest[i] = val
		}
	}

	return nil
}

type libpqResult int64

func (r libpqResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

func (r libpqResult) LastInsertId() (int64, error) {
	return 0, ErrLastInsertId
}
