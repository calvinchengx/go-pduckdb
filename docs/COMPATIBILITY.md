# Compatibility

Supported feature matrix for go-pduckdb. Statuses reflect the current `main` branch.

Legend: ✅ supported · ⚠️ supported with caveats · ❌ not supported (yet)

## Platforms

| Platform | Status | CI-tested | Notes |
|---|---|---|---|
| Linux (amd64/arm64) | ✅ | both | Requires purego ≥ v0.10.0 (struct-by-value support) |
| Linux musl (amd64/arm64) | ✅ | amd64 | DuckDB publishes a musl build; the image must also carry `libstdc++` |
| macOS (amd64/arm64) | ✅ | both | |
| Windows (amd64/arm64) | ✅ | both | Works via an ABI workaround — see [Windows workaround](#windows-workaround) |
| Windows (386/arm) | ❌ | — | Refused at build time; see below |
| FreeBSD / NetBSD | ⚠️ | no | Compiles (covered by build tags), untested |

### Windows workaround

purego does not support passing structs **by value** on Windows, but two DuckDB C
API functions take a `duckdb_result` struct by value:

- `duckdb_fetch_chunk`
- `duckdb_result_return_type`

The driver works around this by relying on a property both 64-bit Windows
calling conventions share: an aggregate too large for registers is passed as a
**hidden pointer** to a caller-allocated copy. On amd64 (Win64) that means
anything other than 1, 2, 4 or 8 bytes; on arm64 (AAPCS64) anything over 16
bytes. `duckdb_result` is 48 bytes, so at the ABI level these functions actually
receive a `duckdb_result *`. The driver registers them with an explicit pointer
argument and wraps them to preserve the by-value signature used on other
platforms (see `internal/duckdb/register_result_windows.go`).

This is an ABI-level workaround, not an officially supported purego feature, so
the assumption it rests on is checked rather than trusted:

- The per-architecture threshold is stated explicitly in
  `internal/duckdb/abi_windows_amd64.go` and `abi_windows_arm64.go`.
- A **compile-time assertion** fails the build if `duckdb_result` is ever
  reduced to a size the convention would pass in registers, because registering
  a by-value function with a pointer argument would then read the wrong memory
  silently. Note this can only check the Go mirror of the struct: the C API
  offers no way to ask for the real size, so a mirror that has drifted from the
  C definition is caught by the tests, not by the assertion.
- 32-bit Windows (`386`, `arm`) **fails to build**. Those conventions push large
  aggregates onto the stack by value, so the workaround would be wrong rather
  than merely unsupported — and DuckDB publishes no 32-bit Windows library.

Both 64-bit Windows targets run the unit tests in CI on every push.

### DLL discovery on Windows

In order: `DUCKDB_LIBRARY_PATH`, `duckdb.dll` beside the **executable**, then in
the current directory, then `%ProgramFiles%\DuckDB\` and
`%ProgramFiles(x86)%\DuckDB\`, and finally the bare name `duckdb.dll` through
the standard `LoadLibrary` search path (`PATH`).

The executable's own directory comes before the working directory because it is
the one location a shipped binary can rely on; the working directory changes
with however the program was launched.

An **absolute** path is loaded with `LoadLibraryExW` restricted to the DLL's own
directory and the process's default directories. That is safer than the legacy
search, which reaches the current directory and `PATH`, and it also lets a
DuckDB DLL resolve dependencies sitting beside it. A bare name still uses the
standard search so that putting a directory on `PATH` keeps working.

## database/sql driver interface

| Feature | Status | Notes |
|---|---|---|
| Query / Exec | ✅ | |
| Prepared statements | ✅ | Positional `?` placeholders; parameter type inference |
| Parameter binding | ✅ | See [Parameter conversions](#parameter-conversions-go--duckdb) |
| Transactions (`Begin`/`Commit`/`Rollback`) | ✅ | `driver.ConnBeginTx` |
| Context support | ⚠️ | Cancellation is checked before execution, not during a running query |
| `Ping` | ✅ | `driver.Pinger` |
| `ColumnTypes()`: scan type | ✅ | `RowsColumnTypeScanType` |
| `ColumnTypes()`: database type name | ✅ | `RowsColumnTypeDatabaseTypeName` (e.g. `INTEGER`, `VARCHAR`) |
| `ColumnTypes()`: nullable | ✅ | `RowsColumnTypeNullable` |
| `ColumnTypes()`: precision & scale | ✅ | `RowsColumnTypePrecisionScale` |
| `Result.RowsAffected()` | ⚠️ | Broken for parameterized `Exec` — see [#23](https://github.com/fpt/go-pduckdb/issues/23) |
| `Result.LastInsertId()` | ❌ | Not supported by DuckDB; returns an error |
| Named parameters | ❌ | Positional only |

## Data types (reading results)

Result values are decoded through DuckDB's data-chunk / vector API.

| DuckDB type | Go value | Notes |
|---|---|---|
| BOOLEAN | `bool` | |
| TINYINT / SMALLINT / INTEGER / BIGINT | `int8` / `int16` / `int32` / `int64` | |
| UTINYINT / USMALLINT / UINTEGER / UBIGINT | `uint8` / `uint16` / `uint32` / `uint64` | |
| HUGEINT | `int64` or `string` | `string` when the value exceeds the int64 range |
| UHUGEINT | `string` | |
| FLOAT / DOUBLE | `float32` / `float64` | |
| DECIMAL | `string` | Exact decimal representation |
| VARINT | `string` | Arbitrary-precision integer, decimal string |
| VARCHAR | `string` | |
| BLOB | `[]byte` | |
| UUID | `string` | |
| ENUM | `string` | Dictionary value |
| DATE / TIME | `time.Time` | |
| TIME WITH TIME ZONE | `time.Time` | Clock component + fixed-offset zone; the date component is not meaningful |
| TIMESTAMP / TIMESTAMP_S / TIMESTAMP_MS / TIMESTAMP_NS | `time.Time` | UTC |
| TIMESTAMP WITH TIME ZONE | `time.Time` | |
| INTERVAL | `string` | |
| LIST / ARRAY | `[]any` | Recursive — nested types decode fully |
| STRUCT | `map[string]any` | Recursive |
| MAP | `[]MapEntry` (`{Key, Value any}`) | Recursive. Note: `MapEntry` is defined in an internal package, so it cannot be referenced by name from user code yet |
| UNION | ❌ | Scans as `NULL` |
| BIT | ❌ | Scans as `NULL` |

## Parameter conversions (Go → DuckDB)

| Go type | DuckDB type |
|---|---|
| `bool` | BOOLEAN |
| `int`, `int8`–`int64`, `uint8`–`uint64` | Numeric types, with range validation |
| `float32`, `float64` | FLOAT / DOUBLE |
| `string` | VARCHAR (or inferred type based on content) |
| `[]byte` | BLOB |
| `time.Time` | DATE, TIME, or TIMESTAMP |

## Requirements

| Requirement | Version | Notes |
|---|---|---|
| Go | ≥ 1.24 | |
| purego | ≥ v0.10.0 | v0.10.0 added struct-by-value arguments on Linux, needed for `duckdb_fetch_chunk` |
| DuckDB shared library | v1.5.x | CI tests against v1.5.4; nearby versions generally work since the C API is stable |
| `libstdc++` | — | Only on musl images (Alpine, distroless static): `libduckdb.so` links against it and a musl base does not carry it |
