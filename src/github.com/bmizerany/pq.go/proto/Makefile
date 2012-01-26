# figure out what GOROOT is supposed to be
GOROOT ?= $(shell printf 't:;@echo $$(GOROOT)\n' | gomake -f -)
include $(GOROOT)/src/Make.inc

TARG=github.com/bmizerany/pq.go/proto
GOFILES=\
	buffer.go\
	msg.go\
	proto.go\
	scan.go\

include $(GOROOT)/src/Make.pkg
