# Builder
FROM golang:alpine
WORKDIR /build
COPY . /build
RUN GOOS=linux go build -o aws-signing-proxy .

# Lean container
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=0 /build/aws-signing-proxy .
EXPOSE 3000
ENTRYPOINT ["/aws-signing-proxy"]
