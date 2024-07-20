VERSION 0.8
FROM golang:1.22-bookworm
WORKDIR /workspace

all:
  COPY +package/*.deb /workspace/dist/
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/*.deb AS LOCAL dist/
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

tidy:
  LOCALLY
  ENV GOTOOLCHAIN=go1.22.1
  RUN go mod tidy
  RUN go fmt ./...

lint:
  FROM golangci/golangci-lint:v1.59.1
  WORKDIR /workspace
  COPY . .
  RUN golangci-lint run --timeout 5m ./...

test:
  COPY go.mod go.sum .
  RUN go mod download
  COPY . .
  RUN go test -coverprofile=coverage.out -v ./...
  SAVE ARTIFACT coverage.out AS LOCAL coverage.out

package:
  FROM debian:bookworm
  # Use bookworm-backports for newer golang versions
  RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/backports.list
  RUN apt update
  # Tooling
  RUN apt install -y git devscripts dpkg-dev debhelper-compat git-buildpackage libfaketime dh-sequence-golang \
    golang-any=2:1.22~3~bpo12+1 golang-go=2:1.22~3~bpo12+1 golang-src=2:1.22~3~bpo12+1
  # Build Dependencies
  RUN apt install -y \
    golang-github-google-btree-dev \
    golang-github-stretchr-testify-dev
  RUN mkdir -p /workspace/golang-github-dpeckett-archivefs
  WORKDIR /workspace/golang-github-dpeckett-archivefs
  COPY . .
  COPY debian/scripts/generate-changelog.sh /usr/local/bin/generate-changelog.sh
  RUN chmod +x /usr/local/bin/generate-changelog.sh
  ENV DEBEMAIL="damian@pecke.tt"
  ENV DEBFULLNAME="Damian Peckett"
  RUN /usr/local/bin/generate-changelog.sh
  RUN VERSION=$(git describe --tags --abbrev=0 | tr -d 'v') \
    && tar -czf ../golang-github-dpeckett-archivefs_${VERSION}.orig.tar.gz .
  RUN dpkg-buildpackage -us -uc
  SAVE ARTIFACT /workspace/*.deb AS LOCAL dist/