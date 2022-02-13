FROM golang:1.17 as builder

ARG appname=proxy

WORKDIR /build
COPY . .
RUN GOOS=linux make build
RUN cp ./bin/${appname} /usr/local/bin/${appname}

FROM centos:7

ARG appname=proxy

COPY --from=builder /build/bin/${appname} /usr/local/bin/${appname}
EXPOSE 8080
CMD ./usr/local/bin/proxy