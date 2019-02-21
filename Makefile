GO := GOPATH=`pwd` go
all: cynic
	./cynic

bluemonday: src/github.com/microcosm-cc/bluemonday/README.md
src/github.com/microcosm-cc/bluemonday/README.md:
	$(GO) get -u github.com/microcosm-cc/bluemonday

markdown: src/github.com/gomarkdown/markdown/README.md
src/github.com/gomarkdown/markdown/README.md:
	$(GO) get -u github.com/gomarkdown/markdown

go-sqlite3: src/github.com/mattn/go-sqlite3/README.md
src/github.com/mattn/go-sqlite3/README.md:
	$(GO) get -u github.com/mattn/go-sqlite3

cynic: cynic.go bluemonday markdown go-sqlite3
	$(GO) build cynic.go

clean:
	rm -fr src pkg

forget-assessments:
	rm -f data/*_*

distclean: clean
	rm -fr data
