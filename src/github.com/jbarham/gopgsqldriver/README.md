pgsqldriver
===========

pgsqldriver is a PostgreSQL driver for the [experimental Go SQL database package]
(https://code.google.com/p/go/source/detail?r=d21caa7909f29cdc).

See https://twitter.com/#!/bradfitz/status/119577116978315264
for the initial announcement of the database driver package, and links
to the first code snapshot and background document.

WARNING
-------

Given that the database package is itself experimental, and pgsqldriver
in turn is an experimental implementation of a driver, it is currently intended
purely to exercise the generic database API and is not intended to be used
in any context where loss of data might cause anxiety.  You have been warned.

Installation
------------

	cd $GOROOT/src/pkg
	git clone git://github.com/jbarham/gopgsqldriver.git github.com/jbarham/gopgsqldriver
	cd github.com/jbarham/gopgsqldriver
	make install

The package `Makefile` assumes that `pg_config` is in your `$PATH` to
automatically determine the location of the PostgreSQL include directory and
the `libpq` shared library.

Usage
-----

	import "exp/sql"
	import _ "github.com/jbarham/gopgsqldriver"
		
Note that by design pgsqldriver is not intended to be used directly.
You do need to import it for the side-effect of registering itself with
the sql package (using the name "postgres") but thereafter all interaction
is via the sql package API.  See the included test file pgsqldriver_test.go
for example usage.

About
-----

pgsqldriver is based on my [pgsql.go package](https://github.com/jbarham/pgsql.go).

John Barham 
jbarham@gmail.com 
[@john_e_barham](http://www.twitter.com/john_e_barham) 
Melbourne, Australia
