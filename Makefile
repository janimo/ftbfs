include $(GOROOT)/src/Make.inc

TARG=ftbfs

GOFILES=\
	helpers.go\
	ftbfs.go\

GC+= -I$(GOPATH)/pkg/$(GOOS)_$(GOARCH)
LD+= -L$(GOPATH)/pkg/$(GOOS)_$(GOARCH)

include $(GOROOT)/src/Make.cmd
