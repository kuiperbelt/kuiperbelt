.PHONY: all

all: deps ekbo plugins

deps:
	go get -d -v ./...

ekbo:
	go build -v -o ./bin/ekbo ./cmd/ekbo

plugins: ekbo-plugin-redispubsub

ekbo-plugin-redispubsub:
	go build -v -o ./bin/ekbo-plugin-redispubsub ./plugin/redispubsub/cmd/ekbo-plugin-redispubsub
