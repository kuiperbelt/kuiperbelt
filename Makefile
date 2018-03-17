CMD_NAME     = ekbo
CMD_PATH     = ./cmd/ekbo
PACKAGE_PATH = github.co/mackee/kuiperbelt

.PHONY: static-build docker-image

static-build:
	cd cmd/ekbo && CGO_ENABLED=0 go build -tags="$(TAGS)" -a -installsuffix cgo -ldflags="-X github.com/mackee/kuiperbelt.Version=$(VERSION)"

docker-image:
	docker build -t kuiperbelt .

include ./_jetpack/jetpack.mk
