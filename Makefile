VERSION := $(shell git describe --tags)

cmd/ekbo/ekbo: *.go cmd/ekbo/main.go
	cd cmd/ekbo && go build -tags="$(TAGS)" -ldflags="-X github.com/mackee/kuiperbelt.Version=$(VERSION)"

.PHONY: clean install get-deps test packages

test:
	go test -v -race
	go vet

get-deps:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure

clean:
	rm -f cmd/ekbo/ekbo

install: cmd/ekbo/ekbo
	install cmd/ekbo/ekbo $(GOPATH)/bin

packages:
	cd cmd/ekbo && gox -os="linux darwin" -arch="amd64 arm" -output "../../pkg/{{.Dir}}-${VERSION}-{{.OS}}-{{.Arch}}" -ldflags "-w -s -X github.com/mackee/kuiperbelt.Version=$(VERSION)"
	cd pkg && find . -name "*${VERSION}*" -type f -exec zip -m -q {}.zip {} \;

release:
	ghr ${VERSION} pkg
