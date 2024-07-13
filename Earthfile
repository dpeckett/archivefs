VERSION 0.8
FROM golang:1.22-bookworm
WORKDIR /workspace

tidy:
  LOCALLY
  RUN go mod tidy
  RUN go fmt ./...

lint:
  FROM golangci/golangci-lint:v1.57.2
  WORKDIR /workspace
  COPY . .
  RUN golangci-lint run --timeout 5m ./...

test:
  RUN apt update && apt install -y iputils-ping
  COPY go.mod go.sum .
  RUN go mod download
  COPY . .
  RUN go test -coverprofile=coverage.out -v ./...
  SAVE ARTIFACT coverage.out AS LOCAL coverage.out