# kuiperbelt

[![Build Status](https://travis-ci.org/mackee/kuiperbelt.svg)](https://travis-ci.org/mackee/kuiperbelt)

Asynchronous Protocol proxy for prefork backend.

## Install & Usage

NOTICE: [README.ja.md](https://github.com/mackee/kuiperbelt/blob/master/README.ja.md) is more details and tutorial in Japanese.

### Installation

```
$ go get github.com/mackee/kuiperbelt/cmd/ekbo
```

### Configuration

Configuration is in YAML format.

Example:
```yaml
port: 12345 # listen port
callback:
  # Callback endpoint for WebSocket connection this useful for authentication.
  # When your application returning "HTTP/1.1 OK 200" in this callback, a connection upgrade to WebSocket.
  # However your returning other than "200" ("404", "403" and "500"...), a connection is disconnect.
  connect: "http://localhost:12346/connect"
proxy_set_header:
  "X-Foo": "Foo"  # set to callback request header
  "X-Bar": ""     # remove from callback request header
```

### Launch

```
$ ekbo -config=config.yml
```

## Configuration

Custimize callback url, port of server and header name of session key.
