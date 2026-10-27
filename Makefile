VERSION ?= dev

.PHONY: build test vet fmt clean

build:
	VERSION=$(VERSION) bash scripts/build.sh

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

clean:
	rm -rf dist
