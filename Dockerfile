############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder
# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache ca-certificates git
WORKDIR $GOPATH/src/github.com/mdelapenya/junit2otlp

COPY . .

# Build the binary.
RUN GOOS=linux GOARCH=386 go build -ldflags="-w -s" -o /go/bin/junit2otlp
############################
# STEP 2 build a small image
############################
FROM scratch
# Copy default certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Copy our static executable.
COPY --from=builder /go/bin/junit2otlp /go/bin/junit2otlp
# Run the junit2otlp binary.
ENTRYPOINT ["/go/bin/junit2otlp"]
CMD [""]
