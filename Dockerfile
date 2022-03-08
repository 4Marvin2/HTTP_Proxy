FROM golang:1.17 AS build

ADD . /app
WORKDIR /app
RUN go build ./main.go

FROM ubuntu:20.04

LABEL org.opencontainers.image.authors="Mikhail Popov"

RUN apt-get -y update && apt-get install -y tzdata
ENV TZ=Russia/Moscow
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

ENV PGVER 12
RUN apt-get -y update && apt-get install -y postgresql-$PGVER
USER postgres

RUN /etc/init.d/postgresql start &&\
    psql --command "CREATE USER marvin WITH SUPERUSER PASSWORD 'vbif';" &&\
    createdb -O marvin http_proxy &&\
    /etc/init.d/postgresql stop


EXPOSE 5432
VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]
USER root

WORKDIR /usr/src/app

COPY . .
COPY --from=build /app/main/ .

EXPOSE 8080
EXPOSE 8000
ENV PGPASSWORD vbif
RUN apt-get install ca-certificates -y
ADD ca.crt /usr/local/share/ca-certificates/ca.crt
RUN chmod 644 /usr/local/share/ca-certificates/ca.crt && update-ca-certificates
CMD service postgresql start && psql -h localhost -d http_proxy -U marvin -p 5432 -a -q -f ./db/dump.sql && ./main