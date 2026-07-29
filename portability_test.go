package srtgo

import (
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// nonWindowsPackages lists packages that have no implementation for
// GOOS=windows. Importing one of them from a test file that is part of the
// Windows build makes the whole test package fail to type-check there, even
// though the library itself builds fine -- i.e. it breaks `go test` on
// Windows for everyone, not just the offending test.
//
// Use the package-internal afINET4/afINET6 (netutils_unix.go,
// netutils_windows.go) instead of unix.AF_INET/unix.AF_INET6.
var nonWindowsPackages = map[string]string{
	"golang.org/x/sys/unix": "use the package-internal afINET4/afINET6 constants",
	"syscall/js":            "wasm only",
}

// TestTestFilesTypeCheckOnWindows guards the property that the test package
// can be type-checked for GOOS=windows. It is a source-level check on purpose:
// cgo cannot be cross-compiled to Windows from a Unix host, so the real
// compiler cannot be used as the guard here.
func TestTestFilesTypeCheckOnWindows(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatalf("glob test files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no test files found; is the working directory the package directory?")
	}

	ctx := build.Default
	ctx.GOOS = "windows"
	ctx.GOARCH = "amd64"
	ctx.CgoEnabled = true

	fset := token.NewFileSet()
	for _, file := range files {
		included, err := ctx.MatchFile(".", file)
		if err != nil {
			t.Fatalf("%s: match for windows: %v", file, err)
		}
		if !included {
			// Explicitly excluded from the Windows build; nothing to check.
			continue
		}

		f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: parse: %v", file, err)
		}
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: bad import path %s: %v", file, imp.Path.Value, err)
			}
			if hint, bad := nonWindowsPackages[path]; bad {
				t.Errorf("%s imports %q, which does not build for GOOS=windows, so `go test` cannot type-check the package there; %s",
					file, path, hint)
			}
		}
	}
}
