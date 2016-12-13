VERSION := $(shell git show -s --format=%h)

cmd/ekbo/ekbo: *.go cmd/ekbo/main.go
	cd cmd/ekbo && go build -tags="$(TAGS)" -ldflags="-X github.com/mackee/kuiperbelt.Version=$(VERSION)"

.PHONY: clean install get-deps test

test:
	go test

get-deps:
	go get -t -d -v .

clean:
	rm -f cmd/ekbo/ekbo

install: cmd/ekbo/ekbo
	install cmd/ekbo/ekbo $(GOPATH)/bin
