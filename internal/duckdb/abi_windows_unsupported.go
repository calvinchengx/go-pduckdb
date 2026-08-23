//go:build windows && !amd64 && !arm64

package duckdb

// go-pduckdb supports windows/amd64 and windows/arm64 only.
//
// The struct-by-value workaround in register_result_windows.go depends on the
// calling convention passing a large aggregate as a hidden pointer. The 32-bit
// Windows conventions push such aggregates onto the stack by value instead, so
// the workaround would read the wrong memory rather than fail. DuckDB also
// publishes no 32-bit Windows library, so there is nothing to load.
//
// Failing to build is deliberate: the alternative is a binary that corrupts
// results at run time.
type windowsArchitectureNotSupported [-1]int
