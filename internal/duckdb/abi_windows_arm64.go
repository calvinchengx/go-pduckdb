package duckdb

// windowsIndirectAggregateThreshold is the largest composite AAPCS64 passes in
// registers. A composite of 16 bytes or less is passed in up to two general
// registers; anything larger is passed indirectly, as a pointer to a
// caller-allocated copy, which is the same property the amd64 workaround relies
// on. Windows on ARM64 follows AAPCS64 for this case.
const windowsIndirectAggregateThreshold = 16
