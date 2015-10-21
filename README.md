# kuiperbelt

Asynchronous Protocol proxy for prefork backend.

## Install & Usage

```
$ go get gopkg.in/mackee/kuiperbelt.v1/cmd/ekbo
$ cat config.yml
port: 12345
callback:
  connect: "http://localhost:12346/connect"
  receive: "http://localhost:12346/receive"
# launch something server within 12346 port.
$ ./ekbo -config=config.yml
```

## Configuration

Custimize callback url, port of server and header name of session key.
