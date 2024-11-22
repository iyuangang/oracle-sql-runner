.PHONY: all build test clean release

BINARY_NAME=sql-runner
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date)
GIT_COMMIT=$(shell git rev-parse HEAD)
LDFLAGS=-X 'github.com/iyuangang/oracle-sql-runner/cmd.Version=${VERSION}' \
        -X 'github.com/iyuangang/oracle-sql-runner/cmd.BuildTime=${BUILD_TIME}' \
        -X 'github.com/iyuangang/oracle-sql-runner/cmd.GitCommit=${GIT_COMMIT}'

all: build

build:
	go build -v -o bin/${BINARY_NAME} -ldflags="${LDFLAGS}"

test:
	go test -v ./...

clean:
	rm -rf bin/
	rm -f ${BINARY_NAME}

release:
	# Linux
	GOOS=linux GOARCH=amd64 go build -o bin/${BINARY_NAME}-linux-amd64 -ldflags="${LDFLAGS}" .\cmd\sql-runner\
	# Windows
	GOOS=windows GOARCH=amd64 go build -o bin/${BINARY_NAME}-windows-amd64.exe -ldflags="${LDFLAGS}" .\cmd\sql-runner\
	# macOS
	GOOS=darwin GOARCH=amd64 go build -o bin/${BINARY_NAME}-darwin-amd64 -ldflags="${LDFLAGS}" .\cmd\sql-runner\
	# 压缩
	cd bin && \
	tar czf ${BINARY_NAME}-linux-amd64.tar.gz ${BINARY_NAME}-linux-amd64 && \
	zip ${BINARY_NAME}-windows-amd64.zip ${BINARY_NAME}-windows-amd64.exe && \
	tar czf ${BINARY_NAME}-darwin-amd64.tar.gz ${BINARY_NAME}-darwin-amd64
