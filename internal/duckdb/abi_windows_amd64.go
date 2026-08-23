package duckdb

// windowsIndirectAggregateThreshold is the largest aggregate the Win64 calling
// convention passes in a register. An aggregate of 1, 2, 4 or 8 bytes is passed
// by value in a register; every other size, including anything larger than 8
// bytes, is passed as a hidden pointer to a caller-allocated copy.
//
// That is what makes the workaround in register_result_windows.go sound: at the
// ABI level, a function declared to take duckdb_result by value actually
// receives a duckdb_result *.
const windowsIndirectAggregateThreshold = 8
