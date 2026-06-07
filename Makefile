BINARY := gdf
PREFIX ?= $(HOME)/.local
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build install uninstall test demo tidy clean git-config help

build: ## Build the gdf binary (stamps version from git describe)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test: ## Run unit tests with the race detector
	go test -race -count=1 ./...

install: build ## Install to ~/.local/bin
	install -Dm755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	@echo "installed -> $(PREFIX)/bin/$(BINARY)"

uninstall: ## Remove installed binary
	rm -f $(PREFIX)/bin/$(BINARY)

tidy: ## go mod tidy
	go mod tidy

demo: build ## Open a sample conflict in the UI
	@mkdir -p .demo
	@printf 'package main\n\nimport "fmt"\n\nfunc greet(name string) string {\n<<<<<<< HEAD\n\treturn fmt.Sprintf("Hello, %%s!", name)\n=======\n\treturn fmt.Sprintf("Hi there, %%s.", name)\n>>>>>>> feature/greeting\n}\n\nfunc main() {\n\tfmt.Println(greet("world"))\n}\n' > .demo/conflict.go
	./$(BINARY) merge .demo/conflict.go

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf .demo

git-config: ## Wire gdf up as a git mergetool
	git config --global mergetool.gdf.cmd 'gdf merge "$$MERGED"'
	git config --global mergetool.gdf.trustExitCode true
	git config --global merge.tool gdf
	@echo "now run: git mergetool"

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
