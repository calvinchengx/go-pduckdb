package duckdb

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

// Loader flags for LoadLibraryExW. Declared here rather than taken from
// golang.org/x/sys so the package keeps no external dependency (#270).
const (
	loadLibrarySearchDLLLoadDir  = 0x00000100
	loadLibrarySearchDefaultDirs = 0x00001000
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	loadLibraryExW = kernel32.NewProc("LoadLibraryExW")
)

// openLibrary loads the DuckDB DLL.
//
// An absolute path goes through LoadLibraryExW with the search restricted to
// the DLL's own directory and the process's default directories. That is both
// safer -- the legacy search reaches the current directory and PATH, which is
// how DLL planting works -- and more correct, because it lets a DuckDB DLL find
// dependencies sitting beside it rather than only ones already on PATH.
//
// A bare name still goes through the standard search, PATH included. Users and
// CI rely on putting the DLL's directory on PATH, and quietly removing that
// would break them; the caller decides by passing a path or a name.
func openLibrary(name string) (uintptr, error) {
	if !filepath.IsAbs(name) {
		// Use [syscall.LoadLibrary] here to avoid external dependencies (#270).
		// For actual use cases, [golang.org/x/sys/windows.NewLazySystemDLL] is recommended.
		handle, err := syscall.LoadLibrary(name)
		return uintptr(handle), err
	}

	wide, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	if err := loadLibraryExW.Find(); err != nil {
		handle, lerr := syscall.LoadLibrary(name)
		return uintptr(handle), lerr
	}
	// wide stays reachable from this frame for the whole call, so the
	// conversion cannot outlive the buffer.
	handle, _, err := loadLibraryExW.Call(
		uintptr(unsafe.Pointer(wide)),
		0,
		loadLibrarySearchDLLLoadDir|loadLibrarySearchDefaultDirs,
	)
	if handle == 0 {
		return 0, err
	}
	return handle, nil
}
