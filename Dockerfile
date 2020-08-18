###########################
# INSTRUCTIONS
############################
# BUILD: docker build -t nsip/otf-reader:develop .
# TEST: docker run -it -v $PWD/test/data:/data -v $PWD/test/config.json:/config.json nsip/otf-reader:develop .
# RUN: docker run -d nsip/otf-reader:develop

#
###########################
# DOCUMENTATION
############################

###########################
# STEP 0 Get them certificates
############################
FROM alpine:latest as certs
RUN apk --no-cache add ca-certificates

############################
# STEP 1 build executable binary (go.mod version)
############################
FROM golang:1.14-stretch as builder
RUN mkdir -p /build
WORKDIR /build
COPY . .
WORKDIR cmd/otf-reader
RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/server

############################
# STEP 2 build a small image
############################
FROM debian:stretch
COPY --from=builder /go/bin/server /go/bin/server
# NOTE - make sure it is the last build that still copies the files
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /go/bin
ENTRYPOINT ["/go/bin/server", "--folder=/data", "--config=/config.json"]
#ENTRYPOINT  "sh", "-c", "echo 'sleep 30'; sleep 30; /go/bin/server --folder=/data --config=/config.json"]
