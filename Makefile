.PHONY: tidy build test

ifdef VERSION
    VERSION=${VERSION}
else
    VERSION=$(shell git describe --always)
endif

GITCOMMIT=$(shell git rev-parse HEAD)
BUILDTIME=${shell date +%Y-%m-%dT%I:%M:%S}

LDFLAGS="-X github.com/narasux/goblog/pkg/version.Version=${VERSION} \
	-X github.com/narasux/goblog/pkg/version.GitCommit=${GITCOMMIT} \
	-X github.com/narasux/goblog/pkg/version.BuildTime=${BUILDTIME}"

# go mod tidy
tidy:
	go mod tidy

# build executable binary
build: tidy
	go build -ldflags ${LDFLAGS} -o goblog ./main.go

# run unittest
test: tidy
	go test ./...

# build docker image
docker:
	docker build -f ./Dockerfile -t goblog:${VERSION} --no-cache .
