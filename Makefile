VERSION=0.0.2
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
all: sabo

.PHONY: sabo

bundle:
	dep ensure

update:
	dep ensure -update

sabo: sabo.go
	go build $(LDFLAGS) -o sabo

linux: sabo.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o sabo

fmt:
	go fmt ./...

clean:
	rm -rf sabo

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
	goreleaser --rm-dist
