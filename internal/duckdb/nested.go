package duckdb

import (
	"fmt"
	"math/big"
	"strings"
	"time"
	"unsafe"
)

// MapEntry is one key/value pair of a decoded MAP value.
type MapEntry struct {
	Key   any
	Value any
}

// listEntry mirrors duckdb_list_entry {uint64 offset; uint64 length}.
type listEntry struct {
	offset uint64
	length uint64
}

// newVector wraps a raw vector handle, caching its data and validity pointers.
func newVector(db *DB, h DuckDBVector) *Vector {
	return &Vector{
		handle:   h,
		db:       db,
		data:     db.VectorGetData(h),
		validity: db.VectorGetValidity(h),
	}
}

// listEntryAt reads the list entry for row i (LIST and MAP share this layout).
func (v *Vector) listEntryAt(i int) listEntry {
	return *(*listEntry)(unsafe.Add(v.data, i*16))
}

// ValueNested decodes any value — scalar or nested — at (col, row) into a
// driver-friendly Go value:
//
//	scalars      -> their natural Go type (see decodeValue)
//	LIST / ARRAY -> []any
//	STRUCT       -> map[string]any
//	MAP          -> []MapEntry
//
// It returns ok=false for a NULL value.
func (r *Result) ValueNested(col int64, row int32) (any, bool) {
	v, off, ok := r.cell(col, row)
	if !ok || !v.RowValid(off) {
		return nil, false
	}
	return decodeValue(r.Db, v, off, r.ColumnLogicalType(col)), true
}

// decodeValue reads the value at index i of vector v, whose logical type is lt.
// Returns nil for NULL. Nested types recurse into their child vectors.
func decodeValue(db *DB, v *Vector, i int, lt DuckDBLogicalType) any {
	if !v.RowValid(i) {
		return nil
	}
	switch db.GetTypeID(lt) {
	case DuckDBTypeBoolean:
		return vectorValue[bool](v, i)
	case DuckDBTypeTinyint:
		return vectorValue[int8](v, i)
	case DuckDBTypeSmallint:
		return vectorValue[int16](v, i)
	case DuckDBTypeInteger:
		return vectorValue[int32](v, i)
	case DuckDBTypeBigint:
		return vectorValue[int64](v, i)
	case DuckDBTypeUTinyint:
		return vectorValue[uint8](v, i)
	case DuckDBTypeUSmallint:
		return vectorValue[uint16](v, i)
	case DuckDBTypeUInteger:
		return vectorValue[uint32](v, i)
	case DuckDBTypeUBigint:
		return vectorValue[uint64](v, i)
	case DuckDBTypeHugeint:
		return hugeintDriverValue(v.hugeintAt(i))
	case DuckDBTypeUHugeint:
		return v.hugeintAt(i).UString()
	case DuckDBTypeFloat:
		return vectorValue[float32](v, i)
	case DuckDBTypeDouble:
		return vectorValue[float64](v, i)
	case DuckDBTypeDecimal:
		return decodeDecimal(db, v, i, lt)
	case DuckDBTypeVarchar:
		return string(v.bytesAt(i))
	case DuckDBTypeBlob:
		return v.bytesAt(i)
	case DuckDBTypeUUID:
		return uuidString(v.hugeintAt(i))
	case DuckDBTypeEnum:
		return decodeEnum(db, v, i, lt)
	case DuckDBTypeDate:
		return dateDaysToTime(vectorValue[int32](v, i))
	case DuckDBTypeTime:
		return timeMicrosToTime(vectorValue[int64](v, i))
	case DuckDBTypeTimeTZ:
		return timeTZToTime(vectorValue[uint64](v, i))
	case DuckDBTypeTimestamp, DuckDBTypeTimestampTZ:
		return timestampMicrosToTime(vectorValue[int64](v, i))
	case DuckDBTypeTimestampS:
		return time.Unix(vectorValue[int64](v, i), 0).UTC()
	case DuckDBTypeTimestampMS:
		return time.UnixMilli(vectorValue[int64](v, i)).UTC()
	case DuckDBTypeTimestampNS:
		return time.Unix(0, vectorValue[int64](v, i)).UTC()
	case DuckDBTypeInterval:
		return v.intervalAt(i).String()
	case DuckDBTypeList:
		return decodeList(db, v, i)
	case DuckDBTypeArray:
		return decodeArray(db, v, i, lt)
	case DuckDBTypeStruct:
		return decodeStruct(db, v, i, lt)
	case DuckDBTypeMap:
		return decodeMap(db, v, i)
	case DuckDBTypeVarInt:
		return decodeVarint(v.bytesAt(i))
	default:
		// UNION, BIT, ... not decoded yet.
		return nil
	}
}

// decodeList decodes a LIST value: the row's list_entry selects a window into
// the shared child vector.
func decodeList(db *DB, v *Vector, i int) []any {
	entry := v.listEntryAt(i)
	childH := db.ListVectorGetChild(v.handle)
	child := newVector(db, childH)
	childLT := db.VectorGetLogicalColumnType(childH)
	defer db.DestroyLogicalType(&childLT)

	out := make([]any, entry.length)
	for k := uint64(0); k < entry.length; k++ {
		out[k] = decodeValue(db, child, int(entry.offset+k), childLT)
	}
	return out
}

// decodeArray decodes a fixed-size ARRAY value: row i occupies a contiguous
// window of size N in the child vector.
func decodeArray(db *DB, v *Vector, i int, lt DuckDBLogicalType) []any {
	n := db.ArrayTypeArraySize(lt)
	childH := db.ArrayVectorGetChild(v.handle)
	child := newVector(db, childH)
	childLT := db.VectorGetLogicalColumnType(childH)
	defer db.DestroyLogicalType(&childLT)

	out := make([]any, n)
	base := i * int(n)
	for k := int64(0); k < n; k++ {
		out[k] = decodeValue(db, child, base+int(k), childLT)
	}
	return out
}

// decodeStruct decodes a STRUCT value into a map keyed by field name.
func decodeStruct(db *DB, v *Vector, i int, lt DuckDBLogicalType) map[string]any {
	cnt := db.StructTypeChildCount(lt)
	out := make(map[string]any, cnt)
	for c := int64(0); c < cnt; c++ {
		name := db.StructTypeChildName(lt, c)
		fieldName := GoString(name)
		if db.Free != nil {
			db.Free(unsafe.Pointer(name))
		}
		childH := db.StructVectorGetChild(v.handle, c)
		child := newVector(db, childH)
		childLT := db.VectorGetLogicalColumnType(childH)
		out[fieldName] = decodeValue(db, child, i, childLT)
		db.DestroyLogicalType(&childLT)
	}
	return out
}

// decodeMap decodes a MAP value. A MAP is physically LIST(STRUCT(key, value)),
// so we walk the list window and pull each entry's "key"/"value" struct fields.
func decodeMap(db *DB, v *Vector, i int) []MapEntry {
	entry := v.listEntryAt(i)
	childH := db.ListVectorGetChild(v.handle)
	child := newVector(db, childH)
	childLT := db.VectorGetLogicalColumnType(childH)
	defer db.DestroyLogicalType(&childLT)

	out := make([]MapEntry, entry.length)
	for k := uint64(0); k < entry.length; k++ {
		kv, _ := decodeValue(db, child, int(entry.offset+k), childLT).(map[string]any)
		out[k] = MapEntry{Key: kv["key"], Value: kv["value"]}
	}
	return out
}

// decodeDecimal reads a DECIMAL as a full-precision decimal string.
func decodeDecimal(db *DB, v *Vector, i int, lt DuckDBLogicalType) string {
	if db.DecimalScale == nil || db.DecimalInternalType == nil {
		return ""
	}
	scale := int(db.DecimalScale(lt))
	switch db.DecimalInternalType(lt) {
	case DuckDBTypeSmallint:
		return formatDecimal(big.NewInt(int64(vectorValue[int16](v, i))), scale)
	case DuckDBTypeInteger:
		return formatDecimal(big.NewInt(int64(vectorValue[int32](v, i))), scale)
	case DuckDBTypeBigint:
		return formatDecimal(big.NewInt(vectorValue[int64](v, i)), scale)
	case DuckDBTypeHugeint:
		return formatDecimal(hugeintToBigInt(v.hugeintAt(i)), scale)
	default:
		return ""
	}
}

// varintHeaderSize is the fixed size of a DuckDB VARINT header (3 bytes).
const varintHeaderSize = 3

// decodeVarint decodes a DuckDB VARINT (arbitrary-precision integer) into its
// decimal string. The physical layout is a string_t: a 3-byte big-endian
// header followed by the magnitude bytes. The header's top bit is the sign
// (1 = positive), and the remaining 23 bits hold the magnitude byte count. For
// negative values both the header and the magnitude bytes are stored as their
// one's complement.
func decodeVarint(b []byte) string {
	if len(b) < varintHeaderSize {
		return ""
	}
	header := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	negative := header&0x800000 == 0
	if negative {
		header = ^header & 0xFFFFFF
	}
	dataLen := int(header & 0x7FFFFF)
	if len(b) < varintHeaderSize+dataLen {
		return ""
	}

	mag := make([]byte, dataLen)
	copy(mag, b[varintHeaderSize:varintHeaderSize+dataLen])
	if negative {
		for i := range mag {
			mag[i] = ^mag[i]
		}
	}
	n := new(big.Int).SetBytes(mag)
	if negative {
		n.Neg(n)
	}
	return n.String()
}

// decodeEnum reads an ENUM value's label from the type's dictionary.
func decodeEnum(db *DB, v *Vector, i int, lt DuckDBLogicalType) string {
	if db.EnumInternalType == nil || db.EnumDictionaryValue == nil {
		return ""
	}
	var idx int64
	switch db.EnumInternalType(lt) {
	case DuckDBTypeUTinyint:
		idx = int64(vectorValue[uint8](v, i))
	case DuckDBTypeUSmallint:
		idx = int64(vectorValue[uint16](v, i))
	case DuckDBTypeUInteger:
		idx = int64(vectorValue[uint32](v, i))
	default:
		return ""
	}
	ptr := db.EnumDictionaryValue(lt, idx)
	if ptr == nil {
		return ""
	}
	s := GoString(ptr)
	if db.Free != nil {
		db.Free(unsafe.Pointer(ptr))
	}
	return s
}

// hugeintDriverValue returns a HUGEINT as int64 when it fits, else its string.
func hugeintDriverValue(h Hugeint) any {
	if (h.Upper == 0 && h.Lower < 1<<63) || (h.Upper == -1 && h.Lower >= 1<<63) {
		return int64(h.Lower)
	}
	return h.String()
}

// dateDaysToTime converts a DuckDB DATE (days since 1970-01-01) to time.Time.
func dateDaysToTime(days int32) time.Time {
	return time.Unix(int64(days)*24*60*60, 0).UTC()
}

// timeMicrosToTime converts a DuckDB TIME (microseconds since midnight) to a
// time.Time on the current UTC date (only the clock component is meaningful).
func timeMicrosToTime(micros int64) time.Time {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.Add(time.Duration(micros) * time.Microsecond)
}

// DuckDB TIME WITH TIME ZONE (dtime_tz_t) packs the time-of-day and its UTC
// offset into a single 64-bit value: the high 40 bits hold microseconds since
// midnight, the low 24 bits hold the UTC offset in seconds biased by
// maxTimeTZOffset (i.e. stored as maxTimeTZOffset - offset).
const (
	timeTZOffsetBits = 24
	timeTZOffsetMask = (1 << timeTZOffsetBits) - 1
	maxTimeTZOffset  = 16*60*60 - 1 // ±15:59:59, the DuckDB offset bias
)

// timeTZToTime converts a DuckDB TIME WITH TIME ZONE value to a time.Time whose
// clock component is the time-of-day and whose location carries the UTC offset.
// As with TIME, only the clock and zone components are meaningful; the date is
// today's UTC date.
func timeTZToTime(bits uint64) time.Time {
	micros := int64(bits >> timeTZOffsetBits)
	offsetSec := maxTimeTZOffset - int32(bits&timeTZOffsetMask)
	zone := time.FixedZone("", int(offsetSec))
	// Use today's UTC date (matching timeMicrosToTime) so the non-semantic date
	// component is stable regardless of the offset; only clock and zone matter.
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, zone)
	return midnight.Add(time.Duration(micros) * time.Microsecond)
}

// timestampMicrosToTime converts a DuckDB TIMESTAMP (microseconds since epoch).
func timestampMicrosToTime(micros int64) time.Time {
	return time.Unix(micros/1_000_000, (micros%1_000_000)*1000).UTC()
}

// String formats an Interval the way DuckDB renders it, e.g.
// "1 year 2 months 3 days 04:05:06".
func (iv Interval) String() string {
	var parts []string
	months := iv.Months
	if years := months / 12; years != 0 {
		parts = append(parts, pluralUnit(int64(years), "year"))
		months %= 12
	}
	if months != 0 {
		parts = append(parts, pluralUnit(int64(months), "month"))
	}
	if iv.Days != 0 {
		parts = append(parts, pluralUnit(int64(iv.Days), "day"))
	}
	micros := iv.Micros
	if micros != 0 || len(parts) == 0 {
		neg := ""
		if micros < 0 {
			neg = "-"
			micros = -micros
		}
		h := micros / 3_600_000_000
		micros %= 3_600_000_000
		m := micros / 60_000_000
		micros %= 60_000_000
		s := micros / 1_000_000
		frac := micros % 1_000_000
		if frac != 0 {
			parts = append(parts, fmt.Sprintf("%s%02d:%02d:%02d.%06d", neg, h, m, s, frac))
		} else {
			parts = append(parts, fmt.Sprintf("%s%02d:%02d:%02d", neg, h, m, s))
		}
	}
	return strings.Join(parts, " ")
}

func pluralUnit(n int64, unit string) string {
	if n == 1 || n == -1 {
		return fmt.Sprintf("%d %s", n, unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
