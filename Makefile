GO := GOPATH=`pwd` go
all: cynic
	./cynic

src/github.com/microcosm-cc/bluemonday/README.md:
	$(GO) get -u github.com/microcosm-cc/bluemonday

src/github.com/russross/blackfriday/README.md:
	$(GO) get -u github.com/russross/blackfriday

src/github.com/coreos/pkg/README.md:
	$(GO) get -u github.com/coreos/pkg/flagutil

src/github.com/dghubble/go-twitter/README.md:
	$(GO) get -u github.com/dghubble/go-twitter/twitter

src/github.com/dghubble/oauth1/README.md:
	$(GO) get -u github.com/dghubble/oauth1

cynic: cynic.go  \
	src/github.com/russross/blackfriday/README.md \
	src/github.com/microcosm-cc/bluemonday/README.md
	$(GO) build cynic.go

getit: getit.go \
	src/github.com/coreos/pkg/README.md \
	src/github.com/dghubble/go-twitter/README.md \
	src/github.com/dghubble/oauth1/README.md
	$(GO) build getit.go

clean:
	rm -fr src pkg

forget-assessments:
	rm -f data/*_*

distclean: clean
	rm -fr data
