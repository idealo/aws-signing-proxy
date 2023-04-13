FROM docker.io/golang:alpine3.16 AS builder
COPY . /build
WORKDIR /build/cmd/aws-signing-proxy
RUN GOOS=linux go build

FROM docker.io/alpine:3.16
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/cmd/aws-signing-proxy/aws-signing-proxy .

RUN addgroup -S proxy && adduser -S proxy -G proxy && chown -R proxy:proxy /app && chmod 777 /app

USER proxy
ENTRYPOINT ["/app/aws-signing-proxy"]
