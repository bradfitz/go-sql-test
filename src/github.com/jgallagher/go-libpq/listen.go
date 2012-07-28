package libpq

/*
#include <stdlib.h>
#include <sys/select.h>
#include <libpq-fe.h>

static PGnotify *waitForNotify(PGconn *conn) {
	int sock;
	fd_set input_mask;
	PGnotify *note;

	sock = PQsocket(conn);
	if (sock < 0) {
		return NULL;
	}

	while (1) {
		FD_ZERO(&input_mask);
		FD_SET(sock, &input_mask);

		// block waiting for input
		if (select(sock+1, &input_mask, NULL, NULL, NULL) < 0) {
			return NULL;
		}

		// check for notifications
		PQconsumeInput(conn);
		if ((note = PQnotifies(conn)) != NULL) {
			return note;
		}
	}
}
*/
import "C"
import (
	"database/sql/driver"
	"unsafe"
)

type libpqListenRows struct {
	c *libpqConn
}

func (r *libpqListenRows) Close() error {
	// we're the exclusive owners of this libpqConn, so it's safe to unlisten *
	_, err := r.c.exec("UNLISTEN *", false)
	return err
}

func (r *libpqListenRows) Columns() []string {
	return []string{"NOTIFICATION"}
}

func (r *libpqListenRows) Next(dest []driver.Value) error {
	// see if we already have pending notifications
	note := C.PQnotifies(r.c.db)
	if note == nil {
		// none pending - block waiting for one
		note = C.waitForNotify(r.c.db)
		if note == nil {
			return driver.ErrBadConn
		}
	}
	defer C.PQfreemem(unsafe.Pointer(note))
	dest[0] = C.GoString(note.extra)
	return nil
}
