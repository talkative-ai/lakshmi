FROM golang:alpine

RUN apk add --update git

COPY docker.gitconfig /root/.gitconfig

RUN go get github.com/artificial-universe-maker/lakshmi

ENTRYPOINT /go/bin/lakshmi

EXPOSE 8080