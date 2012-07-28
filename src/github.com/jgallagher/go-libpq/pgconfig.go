package libpq

/*
// These flags work for building go-libpq on Arch Linux and Mac OS X (with or
// without Postgres.app). To get CFLAGS if these are not sufficient, add
// -I$DIRECTORY where $DIRECTORY is given by
//
//   pg-config --includedir
//
// and set LDFLAGS to -L$DIRECTORY where DIRECTORY is given by
//
//   pg-config --libdir
#cgo darwin CFLAGS: -I/Applications/Postgres.app/Contents/MacOS/include
#cgo darwin LDFLAGS: -L/Applications/Postgres.app/Contents/MacOS/lib
#cgo LDFLAGS: -lpq
*/
import "C"
