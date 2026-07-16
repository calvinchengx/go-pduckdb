package duckdb

import "github.com/ebitengine/purego"

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
