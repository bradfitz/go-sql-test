include $(GOROOT)/src/Make.inc

TARG=github.com/jbarham/pgsqldriver

CGOFILES=\
	pgdriver.go\
	pg_type.go\

CGO_CFLAGS=-I`pg_config --includedir`
CGO_LDFLAGS=-L`pg_config --libdir` -lpq

include $(GOROOT)/src/Make.pkg

