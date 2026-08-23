package duckdb

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// duckdb_result must be large enough that the calling convention passes it
// indirectly; the registrations below depend on it. If the mirror struct is
// ever reduced past that threshold, registering these functions with a pointer
// argument would silently read the wrong memory -- so the build fails instead.
//
// This can only check the Go mirror. The C API exposes no way to ask for the
// real size of duckdb_result, so a mirror that has drifted from the C struct is
// caught by the tests, not by this.
const _ = uint(unsafe.Sizeof(DuckDBResultRaw{}) - windowsIndirectAggregateThreshold - 1)

// registerResultByValueFuncs registers the DuckDB functions that take duckdb_result
// by value. purego does not support struct-by-value arguments on Windows. However, the
// Windows x64 calling convention passes aggregates larger than 8 bytes by a hidden
// pointer to a caller-allocated copy. duckdb_result is 48 bytes, so at the ABI level
// these functions receive a *duckdb_result. We register them with a pointer argument and
// wrap them to preserve the by-value signature the rest of the package expects.
func registerResultByValueFuncs(db *DB, lib uintptr) {
	var fetchChunk func(*DuckDBResultRaw) DuckDBDataChunk
	purego.RegisterLibFunc(&fetchChunk, lib, "duckdb_fetch_chunk")
	db.FetchChunk = func(result DuckDBResultRaw) DuckDBDataChunk {
		return fetchChunk(&result)
	}

	var resultReturnType func(*DuckDBResultRaw) DuckDBResultType
	purego.RegisterLibFunc(&resultReturnType, lib, "duckdb_result_return_type")
	db.ResultReturnType = func(result DuckDBResultRaw) DuckDBResultType {
		return resultReturnType(&result)
	}
}
