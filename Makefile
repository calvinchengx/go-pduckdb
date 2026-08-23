# go-pduckdb — a DuckDB driver for Go that needs no CGO.
#
# These targets are convenience, and they assume GNU make and a POSIX shell:
# Linux, macOS, or WSL / Git Bash on Windows. They are NOT the build.
#
# The build and the tests are the Go toolchain alone and run natively on all
# three, on amd64 and arm64 -- CI proves it on six targets, and it invokes `go`
# directly rather than make for exactly that reason. On Windows:
#
#     go test ./...
#
# with duckdb.dll resolvable (see "Library Path Configuration" in the README).
# `make help` and the docker targets are the parts that want a POSIX shell.

.PHONY: run test fmt lint integ integ-arm64 integ-musl integ-musl-arm64 help

run: ## Run the application
	CGO_ENABLED=0 go run example/simple/main.go
	CGO_ENABLED=0 go run example/databasesql/main.go
	CGO_ENABLED=0 go run example/databasesql2/main.go
	CGO_ENABLED=0 go run example/columntypes/main.go
	CGO_ENABLED=0 go run example/enhancedtypes/main.go
	CGO_ENABLED=0 go run example/json/main.go
	CGO_ENABLED=0 go run example/multistatement/main.go

test: ## Run unit tests
	go test -v ./...

fmt: ## Run format
	gofumpt -extra -w .

lint: ## Run lint
	golangci-lint run

integ: ## Run integration tests
	docker build --platform linux/amd64 -t go-pduckdb/integ -f internal/integ/Dockerfile . && \
	docker run --rm go-pduckdb/integ

integ-arm64: ## Run integration tests on arm64
	docker build --platform linux/arm64 --build-arg GOARCH=arm64 --build-arg LIBARCH=arm64 -t go-pduckdb/integ-arm64 -f internal/integ/Dockerfile . && \
	docker run --rm go-pduckdb/integ-arm64

integ-musl: ## Run integration tests against the musl build of DuckDB
	docker build --platform linux/amd64 -t go-pduckdb/integ-musl -f internal/integ/Dockerfile.musl . && \
	docker run --rm go-pduckdb/integ-musl

integ-musl-arm64: ## Run integration tests against the musl build on arm64
	docker build --platform linux/arm64 --build-arg GOARCH=arm64 --build-arg LIBARCH=arm64 -t go-pduckdb/integ-musl-arm64 -f internal/integ/Dockerfile.musl . && \
	docker run --rm go-pduckdb/integ-musl-arm64

help: ## Display this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'
