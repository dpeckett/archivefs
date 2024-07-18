VERSION 0.8
FROM golang:1.21-bookworm
WORKDIR /workspace

all:
  COPY +package/*.deb /workspace/dist/
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/*.deb AS LOCAL dist/
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

tidy:
  LOCALLY
  ENV GOTOOLCHAIN=go1.21.8 
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
  RUN apt install -y devscripts dpkg-dev debhelper-compat dh-sequence-golang golang-any golang-1.21 git
  # Build Dependencies
  RUN apt install -y golang-github-google-btree-dev golang-github-stretchr-testify-dev
  RUN mkdir -p /workspace/golang-github-dpeckett-archivefs
  WORKDIR /workspace/golang-github-dpeckett-archivefs
  COPY . .
  ENV EMAIL=damian@pecke.tt
  RUN export DEBEMAIL="damian@pecke.tt" \
    && export DEBFULLNAME="Damian Peckett" \
    && export VERSION=$(git describe --tags --abbrev=0 | tr -d 'v') \
    && dch --create --package golang-github-dpeckett-archivefs --newversion "${VERSION}-1" \
      --distribution "UNRELEASED" --force-distribution  --controlmaint "Last Commit: $(git log -1 --pretty=format:'(%ai) %H %cn <%ce>')" \
    && tar -czf ../golang-github-dpeckett-archivefs_${VERSION}.orig.tar.gz .
  RUN dpkg-buildpackage -us -uc
  SAVE ARTIFACT /workspace/*.deb AS LOCAL dist/
