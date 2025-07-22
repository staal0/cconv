OUTPUT?="cconv"
VERSION=$(shell git describe --tags)
LDFLAGS="-X main.Build=$(VERSION)"

.PHONY: build install clean

build:
	CGO_ENABLED=0 go build -o $(OUTPUT) -ldflags=$(LDFLAGS)

install:
	CGO_ENABLED=0 go install -ldflags=$(LDFLAGS)

clean:
	rm -rf dist
