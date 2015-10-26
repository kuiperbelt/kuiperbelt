# kuiperbelt

[![Build Status](https://travis-ci.org/mackee/kuiperbelt.svg)](https://travis-ci.org/mackee/kuiperbelt)

Asynchronous Protocol proxy for prefork backend.

## Install & Usage

```
$ go get github.com/mackee/kuiperbelt/cmd/ekbo
$ cat config.yml
port: 12345
callback:
  connect: "http://localhost:12346/connect"
# launch something server within 12346 port.
$ ./ekbo -config=config.yml
```

## Configuration

Custimize callback url, port of server and header name of session key.
