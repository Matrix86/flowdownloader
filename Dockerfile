FROM golang:alpine AS build-env

RUN apk add --update ca-certificates
RUN apk add --no-cache --update make

WORKDIR /go/src/app

COPY . .

RUN go get -d -v ./...

RUN make

FROM alpine:latest

COPY --from=build-env /go/src/app/flowdownloader /app/

WORKDIR /app

ENTRYPOINT ["/app/flowdownloader"]