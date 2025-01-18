VERSION=0.4.3
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"

all: sabo

.PHONY: sabo

sabo: sabo.go
	go build $(LDFLAGS) -o sabo

linux: sabo.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o sabo

check:
	go test ./...

fmt:
	go fmt ./...

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
