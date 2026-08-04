GO ?= go
BIN ?= workgraph
PKG := github.com/moul/workgraph/cmd/workgraph

.PHONY: test
test:
	$(GO) test ./...

.PHONY: run
run:
	$(GO) run $(PKG) $(ARGS)

.PHONY: install
install:
	$(GO) install $(PKG)

.PHONY: deps
deps:
	$(GO) mod tidy

.PHONY: lint
lint:
	$(GO) vet ./...
	gofmt -l -s .

.PHONY: fmt
fmt:
	gofmt -w -s .

.PHONY: clean
clean:
	rm -f $(BIN) wg
	$(GO) clean ./...
