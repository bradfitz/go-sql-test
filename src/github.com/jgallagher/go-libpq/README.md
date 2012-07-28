# libpq - cgo-based Postgres driver for Go's database/sql package

## Install

If your Postgres headers and libraries are installed in what appears to be
the typical places:

	/usr/include/libpq-fe.h
	/usr/lib/libpq.so     # on Linux
	/usr/lib/libpq.dynlib # on Mac os X

or you're on Mac OS X and have installed [Postgres.app](http://postgresapp.com/)
in /Applications, then

	go get github.com/jgallagher/go-libpq

should work. If you have build problems, you will need to modify pgconfig.go to
point to the correct locations. See that file for instructions. (Please let me
know if there's a way I could make this smoother; [this
discussion](https://groups.google.com/forum/#!msg/golang-nuts/ABK6gcHbBjc/eGlxjrmXzfoJ)
seems to imply that there isn't much support for this sort of thing at the
moment.)

## Use

```go
package main

import (
	_ "github.com/jgallagher/go-libpq"
	"database/sql"
)

func main() {
	db, err := sql.Open("libpq", "user=USERNAME dbname=gosqltest sslmode=disable")
	// ...
}
```

The connection string passed to Open() is passed through with no changes
to the [PQconnectdb](http://www.postgresql.org/docs/9.1/static/libpq-connect.html)
function from Postgres; see their documentation for supported parameters.

## LISTEN/NOTIFY Support

There is no explicit support for NOTIFY; simply calling `Exec("NOTIFY channel,
message")` is sufficient. LISTEN is a different beast. This driver allows for
support for LISTEN completely within the database/sql API, but some care must
be taken to avoid undetectable (by the go runtime) deadlock. Specifically,
to start listening on a channel, issue a LISTEN Query(), and then call
Next()/Scan() on the returned sql.Rows to wait for notifications:

```go
// assuming "db" was returned from sql.Open(...)
notifications, err := db.Query("LISTEN mychan")
if err != nil {
	// handle "couldn't start listening"
}

// wait for a notification to arrive on channel "mychan"
// WARNING: This call will BLOCK until a notification arrives!
if !notifications.Next() {
	// this will never happen unless there is a failure with the underlying
	// database connection
}

// get the message sent on the channel (possibly "")
var message string
notifications.Scan(&message)
```

It's almost certain that the actual use for this will be inside a goroutine
that relays notifications back on a channel. For a full example, see
`examples/listen_notify.go` in the repository.

## Testing

To run the tests, just run `go test -v`. A test database must be set up;
it uses exactly the same database configuration as https://github.com/bradfitz/go-sql-test/.
Create the database `gosqltest`, and give yourself ($USER) privileges with
the password `gosqltest`.

This driver passes everything in go-sql-test, but has not yet been submitted
for inclusion in that repository.
