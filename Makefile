GO ?= go
GOLANGCI_LINT ?= $$($(GO) env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.60.3

BUILD_DATE="$(shell date +'%Y-%m-%d')"
VERSION="$(shell date +'%m%y')"
HASH="$(shell git rev-parse HEAD)"


.PHONY: test
test:
	$(GO) test ./...

.PHONY: lint
lint: linter
	$(GOLANGCI_LINT) run

.PHONY: linter
linter:
	test -f $(GOLANGCI_LINT) || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$($(GO) env GOPATH)/bin $(GOLANGCI_LINT_VERSION)


.PHONY: format
format:
	$(GO) fmt ./...

.PHONY: run
run:
	$(GO) run ./cmd/cli-client

.PHONY: build
build:
	$(GO) build -o bin/tinytorrent \
	-ldflags="-X 'github.com/Despire/tinytorrent/p2p/client/internal/build.Date=${BUILD_DATE}'\
	-X 'github.com/Despire/tinytorrent/p2p/client/internal/build.Version=${VERSION}'\
	-X 'github.com/Despire/tinytorrent/p2p/client/internal/build.Hash=${HASH}'"  ./cmd/cli-client