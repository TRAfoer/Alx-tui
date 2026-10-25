BINARY  ?= bin/alx-tui
GO      ?= go
GOFLAGS ?=
ARGS    ?=

.PHONY: all build run test vet fmt clean

all: build

build:
	mkdir -p $(dir $(BINARY))
	$(GO) build $(GOFLAGS) -trimpath -ldflags "-s -w" -o $(BINARY) .

run: build
	./$(BINARY) $(ARGS)

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w main.go internal

clean:
	rm -f $(BINARY)
