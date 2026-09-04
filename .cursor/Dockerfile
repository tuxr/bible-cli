FROM golang:1.26-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# golangci-lint version must match .github/workflows/ci.yml
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 \
    && cp "$(go env GOPATH)/bin/golangci-lint" /usr/local/bin/golangci-lint \
    && golangci-lint version
