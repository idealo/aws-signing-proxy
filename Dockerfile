# Builder
FROM golang:alpine
COPY . /build
WORKDIR /build/cmd/aws-signing-proxy
RUN GOOS=linux go build

# Lean container
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=0 /build/cmd/aws-signing-proxy/aws-signing-proxy .

RUN addgroup -S proxy && adduser -S proxy -G proxy
RUN chown -R proxy:proxy /app
RUN chmod 750 /app

EXPOSE 3000

USER proxy
ENTRYPOINT ["/app/aws-signing-proxy"]
