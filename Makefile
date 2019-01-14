all: cynic
	./cynic

src/github.com/microcosm-cc/bluemonday/README.md:
	GOPATH=`pwd` go get -u github.com/microcosm-cc/bluemonday

src/github.com/russross/blackfriday/README.md:
	GOPATH=`pwd` go get -u github.com/russross/blackfriday

cynic: cynic.go src/github.com/russross/blackfriday/README.md src/github.com/microcosm-cc/bluemonday/README.md
	GOPATH=`pwd` go build cynic.go

clean:
	rm -fr src pkg

distclean: clean
	rm -fr data
