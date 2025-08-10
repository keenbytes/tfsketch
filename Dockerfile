FROM golang:alpine AS builder
LABEL maintainer="Mikołaj Gąsior"

RUN apk add --update git bash openssh make gcc musl-dev

WORKDIR /go/src/keenbytes/tfsketch
COPY . .
RUN go build -o tfsketch

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /bin
COPY --from=builder /go/src/keenbytes/tfsketch/tfsketch tfsketch
RUN chmod +x /bin/tfsketch
RUN /bin/tfsketch
ENTRYPOINT ["/bin/tfsketch"]
