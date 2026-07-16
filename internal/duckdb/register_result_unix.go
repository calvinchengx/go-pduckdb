//go:build darwin || freebsd || linux || netbsd

package duckdb

import "github.com/ebitengine/purego"

// registerResultByValueFuncs registers the DuckDB functions that take duckdb_result
// by value. On darwin/linux (amd64/arm64) purego supports passing structs by value
// directly, so the fields can be registered as-is.
func registerResultByValueFuncs(db *DB, lib uintptr) {
	purego.RegisterLibFunc(&db.FetchChunk, lib, "duckdb_fetch_chunk")
	purego.RegisterLibFunc(&db.ResultReturnType, lib, "duckdb_result_return_type")
}
