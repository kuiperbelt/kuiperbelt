# kuiperbelt

[![Build Status](https://travis-ci.org/mackee/kuiperbelt.svg)](https://travis-ci.org/mackee/kuiperbelt)

The proxy server that converts WebSocket to HTTP/1.x.

## Install & Usage

NOTICE: [README.ja.md](https://github.com/mackee/kuiperbelt/blob/master/README.ja.md) is more details and tutorial in Japanese.

### Installation

I recommend installing from [binary releases](https://github.com/mackee/kuiperbelt/releases).

This installation easy and well to do using [Songmu/ghg](https://github.com/Songmu/ghg).

```console
$ ghg get mackee/kuiperbelt
$ export PATH=$(ghg bin):$PATH
```

Also, you can use the kuiperbelt in docker.

```
$ docker pull mackee/kuiperbelt:latest
```

### Configuration

A configuration is in YAML format.

```yaml
port: 12345 # listen to the port
sock: "" # If set sock path this option, a kuiperbelt is to use UNIX domain socket.
callback:
  # A callback endpoint for starts WebSocket connection this useful for authentication.
  # When your application returning "HTTP/1.1 OK 200" in this callback, a connection upgrade to WebSocket.
  # However your callback returning other than "200" ("404", "403" and "500"...), this connection is a disconnect.
  connect: "http://localhost:12346/connect"
  # A close callback is similar working to the connect callback. A kuiperbelt send request this callback when closed connection by a client or timeout of idle(if set `idle_timeout`).
  close: "http:/localhost:12346/close"
  timeout: 10s    # timeout of callback response
# A log level of access log is `info`. But suppress this when this option is true.
suppress_access_log: false 
# This option can change a header name of session id.
session_header: "X-Kuiperbelt-Session"
# An "X-Kuiperbelt-Endpoint" header in connect callback is indicating an endpoint of kuiperbelt.
# Your application can use this value when multi-host of kuiperbelt.
# By default, this value is from `hostname` command. But if you can not use the value from `hostname`(ex. in docker), by using this option you can set suitable values.
endpoint: "localhost:12345"
proxy_set_header:
  "X-Foo": "Foo"  # set to callback request header
  "X-Bar": ""     # remove from callback request header
send_timeout: 0    # timeout of sending a message to a client. 0 is off.
send_queue_size: 0 # queue size of message to client. this value is per a cliet.
# This option is to use for checking Origin header.
# If set `none`, not check.
# If set `same_origin`, check equals Origin to Host.
# If set `same_hostname`, check equals hostname of Origin to hostname of Host, ignoring port.
origin_policy: none
# If the idle state continues this value, disconnect automatically.
# In this case, working close callback. 0 is disable this feature.
idle_timeout: 0 
```

Also, if you using docker image, you can set these options by environment variables.

The corresponding environment variable is following [_example/config_by_env.yml](https://github.com/mackee/kuiperbelt/blob/master/_example/config_by_env.yml).

### Launch

The launch with specifies configuration file.
```
$ ekbo -config=config.yml
```

Or, launch in docker.

```
docker run -p 9180:9180 -e EKBO_CONNECT_CALLBACK_URL=http://yourwebapp/connect mackee/kuiperbelt
```

Also, an example application in [_example](https://github.com/mackee/kuiperbelt/blob/master/_example). You can launch this application by docker-compose.

## Features

### Web API

#### for client(ex. browser, native apps)

- GET `/connect` - starts WebSocket connection
  - You can use the query string or header for authentication. These values pass through to the connect callback.

#### for backend application

- POST `/send` - send message to connection of WebSocket
  - `X-Kuiperbelt-Session` in request header: target session id
  - request body: pass through to a client by WebSocket.
- POST `/close` - close connection of WebSocket
  - `X-Kuiperbelt-Session` in request header: target session id
  - request body: pass through to a client by WebSocket. useful to goodbye message.

#### for monitoring

- GET `/ping` - useful for the health check.
- GET `/stats` - metrics of kuiperbelt. living connections, error rate, etc...

### Callback

The callback is similar to webhook.

- `connect` callback - request when starts WebSocket.
  - response body: pass through to a client by WebSocket. useful to hello message.
- `close` callback - request when closed connection by client or idle.

## Author

* [mackee](https://github.com/mackee)
* [shogo82148](https://github.com/shogo82148)
* [fujiwara](https://github.com/fujiwara)

## License

[The MIT License](https://github.com/mackee/kuiperbelt/blob/master/LICENCE)

Copyright (c) 2015 TANIWAKI Makoto / (c) 2015 [KAYAC Inc.](https://github.com/kayac)
