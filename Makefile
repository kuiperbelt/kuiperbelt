VERSION := $(shell git show -s --format=%h)

cmd/ekbo/ekbo: *.go cmd/ekbo/main.go
	cd cmd/ekbo && go build -ldflags="-X github.com/mackee/kuiperbelt.Version=$(VERSION)"

.PHONY: clean install get-deps

get-deps:
	go get .

clean:
	rm -f cmd/ekbo/ekbo

install: cmd/ekbo/ekbo
	install cmd/ekbo/ekbo $(GOPATH)/bin
