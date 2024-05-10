#FROM golang:latest AS build-env

# Build Delve
#RUN go install github.com/go-delve/delve/cmd/dlv@latest

#ADD . /dockerdev
#WORKDIR /dockerdev/cmd/bgp-dns

#RUN go build -gcflags="all=-N -l" -o /server

# Final stage
FROM debian:bookworm

EXPOSE 179 1179
ENV DEBIAN_FRONTEND=noninteractive

WORKDIR /app

#COPY --from=build-env /go/bin/dlv /
#COPY --from=build-env /server /

#CMD ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "/server"]