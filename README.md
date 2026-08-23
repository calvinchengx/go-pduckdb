# go-pduckdb is a PureGO driver for [DuckDB](https://duckdb.org/docs/stable/clients/c/api.html)

> **This is a fork** of [fpt/go-pduckdb](https://github.com/fpt/go-pduckdb) by Youichi
> Fujimoto (MIT), maintained so that Linux, macOS and Windows are all first-class:
> windows/arm64 support, a compile-time check on the Windows struct-by-value ABI
> workaround, macOS amd64 and Linux arm64 back in CI, and a musl build proven
> rather than assumed. See [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).
>
> The module path is `github.com/calvinchengx/go-pduckdb`; everything else
> matches upstream. Changes are offered back — see
> [fpt/go-pduckdb#37](https://github.com/fpt/go-pduckdb/pull/37) — and this fork
> exists to be merged out of existence if they land.

## Introduction

A DuckDB module for Go which doesn't require CGO.
Uses [purego](https://github.com/ebitengine/purego) to interface with DuckDB's native library.

## Why go-pduckdb?

Existing DuckDB drivers for Go rely on CGO and compile or link DuckDB into your binary. go-pduckdb is an independent implementation that takes a different approach: no CGO, loading the DuckDB shared library at runtime via [purego](https://github.com/ebitengine/purego). This gives you:

- **`CGO_ENABLED=0` builds** — no C toolchain, simple cross-compilation, works in build environments where CGO is unavailable or unwanted
- **Fast builds and small binaries** — DuckDB is not compiled or linked into your binary
- **DuckDB upgrades without recompiling** — swap the shared library (e.g. `brew upgrade duckdb`) and your existing binary uses it

In short, go-pduckdb moves the DuckDB dependency from build time to run time — your program needs `libduckdb` present on the machine where it runs (see [Installation](#installation)).

## Features

- Pure Go implementation - no CGO required
- Support for most DuckDB data types including DATE, TIME, TIMESTAMP, and DECIMAL
- SQL query execution and result handling
- Database access through standard database/sql interface
- Clear error reporting and propagation
- Cross-platform compatibility
- Parameter binding with automatic type conversion
- Support for prepared statements with parameter type inference
- Transaction support

See [docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md) for the full supported feature matrix (platforms, database/sql interfaces, and data types).

## Installation

```bash
go get github.com/calvinchengx/go-pduckdb
```

Also, make sure to install DuckDB on your platform:

### macOS
```bash
brew install duckdb
```

Typically, `/opt/homebrew/lib/libduckdb.dylib` is installed.

### Linux (Ubuntu/Debian)
```bash
curl -sSL https://github.com/duckdb/duckdb/releases/download/v1.5.4/libduckdb-linux-amd64.zip -o archive.zip
sudo unzip -j archive.zip libduckdb.so -d /usr/local/lib
sudo ldconfig
rm archive.zip
```

You can find a download URL in [official releases of DuckDB](https://github.com/duckdb/duckdb/releases).
Assets starting with `libduckdb-` contains glibc build of `libduckdb.so`.

For other Linux, Check official instruction: [Building DuckDB](https://duckdb.org/docs/stable/dev/building/linux.html).

### Windows
Download the DuckDB CLI from the [official website](https://duckdb.org/docs/installation/) and place the DLL in your system path.

Note: Windows support relies on an ABI-level workaround for purego's lack of struct-by-value arguments on Windows — see the [Windows workaround](./docs/COMPATIBILITY.md#windows-workaround) section in the compatibility docs. Both amd64 and arm64 are supported and both run the unit tests in CI on every push.

Once `duckdb.dll` is resolvable, the tests are the Go toolchain and nothing else:

```powershell
go test ./...
```

The `Makefile` is convenience rather than the build — its targets assume GNU make and a POSIX shell, so on Windows they want WSL or Git Bash. Nothing is lost by skipping it: `make test` is `go test ./...`, and the remaining targets build Docker images or run `gofumpt` and `golangci-lint`. CI invokes `go` directly for the same reason, which is what makes the Windows results mean anything.

## Library Path Configuration

go-pduckdb searches for the DuckDB library in several locations. You can configure the search path using environment variables:

- `DUCKDB_LIBRARY_PATH` - specify the exact path to the DuckDB library file
- `DYLD_LIBRARY_PATH` - on macOS, specify directories to search for the DuckDB library
- `LD_LIBRARY_PATH` - on Linux, specify directories to search for the DuckDB library

Example usage:

```bash
# Specify exact library path
DUCKDB_LIBRARY_PATH=/path/to/libduckdb.dylib ./your_program

# Or specify directory to search (macOS)
DYLD_LIBRARY_PATH=/path/to/lib ./your_program

# Or specify directory to search (Linux)
LD_LIBRARY_PATH=/path/to/lib ./your_program
```

If no environment variables are set, the library will be searched in standard system locations.

## Usage Examples

### Using Standard database/sql Interface

go-pduckdb implements the Go standard database/sql interface, allowing you to work with DuckDB like any other SQL database in Go:

```go
package main

import (
	"database/sql"
	"fmt"
	"log"
	
	_ "github.com/calvinchengx/go-pduckdb" // Import for driver registration
)

func main() {
	// Open a database connection
	db, err := sql.Open("duckdb", "example.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	
	// Create a table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY, 
		name VARCHAR, 
		email VARCHAR
	)`)
	if err != nil {
		log.Fatal(err)
	}
	
	// Insert data
	_, err = db.Exec(`INSERT INTO users (id, name, email) VALUES (?, ?, ?)`, 
		1, "John Doe", "john@example.com")
	if err != nil {
		log.Fatal(err)
	}
	
	// Query data
	rows, err := db.Query("SELECT id, name, email FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	
	// Process results
	for rows.Next() {
		var id int
		var name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("User %d: %s (%s)\n", id, name, email)
	}
}
```

For a more comprehensive example, see the [database/sql example](./example/databasesql/main.go).

### Parameter Binding and Type Conversion

go-pduckdb features a sophisticated type conversion system that automatically handles type conversions for prepared statement parameters:

```go
// Prepare a statement
stmt, err := conn.Prepare("INSERT INTO users (id, name, created_date) VALUES (?, ?, ?)")
if err != nil {
    log.Fatal(err)
}
defer stmt.Close()

// Execute with different parameter types
// The driver will automatically convert these to the appropriate types
err = stmt.Execute(
    1,                                 // int -> INTEGER
    "John Doe",                        // string -> VARCHAR
    time.Date(2025, 5, 3, 0, 0, 0, 0, time.UTC),  // time.Time -> DATE
)
```

Supported conversions include:
- Go bool -> DuckDB BOOLEAN
- Go numeric types -> DuckDB numeric types with range validation
- Go string -> Various DuckDB types based on content
- Go []byte -> DuckDB BLOB
- Go time.Time -> DuckDB DATE, TIME, or TIMESTAMP
- Custom Date, Time, and Interval types for precise control

For more examples, check the [example](./example) directory.

## API Documentation

### Standard database/sql Interface

go-pduckdb registers itself as a driver named "duckdb" with the standard database/sql package, supporting:

- Connection management (Open, Close)
- Query execution (Exec, Query)
- Prepared statements
- Transactions
- Context handling
- Parameter binding

### Native API

go-pduckdb also provides a native API for more direct interaction with DuckDB:

- **DuckDB**: Represents a database instance
- **DuckDBConnection**: Handles connections to the database
- **DuckDBResult**: Manages query results
- **DuckDBDate**, **DuckDBTime**, **DuckDBTimestamp**: Date and time types

### Date and Time Handling

go-pduckdb provides native Go type conversions for DuckDB's date and time types:

```go
// Get date value
dateVal, hasValue := result.ValueDate(columnIndex, rowIndex)
if hasValue {
    fmt.Println("Date:", dateVal.Format("2006-01-02"))
}

// Get timestamp value
tsVal, hasValue := result.ValueTimestamp(columnIndex, rowIndex)
if hasValue {
    fmt.Println("Timestamp:", tsVal.Format("2006-01-02 15:04:05.000000"))
}
```

## Type support

Result values are read through DuckDB's data-chunk / vector API — the modern,
non-deprecated path — rather than the deprecated `duckdb_value_*` accessors.
This covers:

- All scalar types, including **BLOB** and **INTERVAL** (previously blocked by
  purego's struct-return limits), plus DECIMAL, UUID, HUGEINT and ENUM.
- The nested types **LIST**, **ARRAY**, **STRUCT** and **MAP**, decoded to
  `[]any`, `map[string]any` and `[]duckdb.MapEntry` respectively (recursively,
  so lists of structs etc. work).

See [docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md) for the full per-type matrix.

### Unsupported types (yet)

These currently scan as `NULL`:

- UNION
- BIT / VARINT
- TIME WITH TIME ZONE

### Requirements

- **purego v0.10.0 or newer.** v0.10.0 added struct-by-value argument support on
  Linux; the driver needs it to call `duckdb_fetch_chunk` (which takes the
  `duckdb_result` struct by value) on non-macOS platforms. Earlier purego
  releases only allowed struct arguments on darwin.

## Project Structure

This project follows the [standard Go project layout](https://go.dev/doc/modules/layout):

```
go-pduckdb/
├── driver.go        # database/sql driver implementation
├── error.go         # Error handling
├── pduckdb.go       # Core public API (DuckDB, Connect, Close)
├── *_test.go        # Unit + integration tests
├── example/         # Example programs
│   ├── columntypes/     # Column type demonstration
│   ├── databasesql/     # database/sql usage examples
│   ├── databasesql2/    # Additional database/sql examples
│   ├── enhancedtypes/   # Enhanced type support examples
│   ├── json/            # JSON handling examples
│   ├── multistatement/  # Multi-statement examples
│   └── simple/          # Simple API usage examples
└── internal/        # Internal implementation
    ├── convert/         # Parameter type-conversion utilities
    ├── duckdb/          # Low-level DuckDB bindings
    │   ├── library.go       # Library loading (purego Dlopen)
    │   ├── db.go            # C function registration
    │   ├── conn.go          # Connection handling
    │   ├── statement.go     # Prepared statements + parameter binding
    │   ├── result.go        # Result reading (data-chunk cache)
    │   ├── chunk.go         # Data-chunk / vector decoding
    │   └── type.go          # DuckDB type definitions
    └── integ/           # Integration test infrastructure
```

## Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md) before submitting a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## References

### DuckDB

- [Official Documentation](https://duckdb.org/docs/stable/clients/c/api.html)
- [C API Source](https://github.com/duckdb/duckdb/tree/main/src/main/capi)
- [C Header](https://github.com/duckdb/duckdb/tree/main/src/include/duckdb.h)
