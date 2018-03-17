FROM golang:1.10 AS build

WORKDIR /work
ENV GOPATH /work/gocode
RUN mkdir -p $GOPATH/src $GOPATH/pkg $GOPATH/bin
ADD . $GOPATH/src/github.com/mackee/kuiperbelt
ENV PATH $GOPATH/bin:$PATH
RUN cd $GOPATH/src/github.com/mackee/kuiperbelt && make clean && go get -u github.com/golang/dep/cmd/dep && make get-deps && make static-build

FROM scratch

COPY --from=build /work/gocode/src/github.com/mackee/kuiperbelt/cmd/ekbo/ekbo /usr/local/bin/ekbo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /work/gocode/src/github.com/mackee/kuiperbelt/_example/config_by_env.yml $WORKDIR/config.yml
ENTRYPOINT ["/usr/local/bin/ekbo"]
