CMD_NAME     = ekbo
CMD_PATH     = ./cmd/ekbo
PACKAGE_PATH = github.com/kuiperbelt/kuiperbelt

.PHONY: static-build docker-image

static-build:
	cd cmd/ekbo && CGO_ENABLED=0 go build -tags="$(TAGS)" -a -installsuffix cgo -ldflags="-X github.com/kuiperbelt/kuiperbelt.Version=$(VERSION)"

docker-image:
	docker build -t kuiperbelt .

include ./_jetpack/jetpack.mk
