package testutils

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

func FilePath(relative string, up ...int) string {
	offset := 1
	if len(up) > 0 && up[0] > 0 {
		offset = up[0]
	}
	_, file, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}
	if filepath.IsAbs(relative) {
		panic(fmt.Sprintf("%s is an absolute file path, must be relative to the current go source file", relative))
	}
	abs := filepath.Join(filepath.Dir(file), relative)
	return abs
}

func FuncSignature(up ...int) string {
	offset := 1
	if len(up) > 0 && up[0] > 0 {
		offset = +up[0]
	}
	pc, _, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}

	f := runtime.FuncForPC(pc)
	if f == nil {
		panic("failed to get function name")
	}
	return f.Name()
}

func FuncName(up ...int) string {
	offset := 1
	if len(up) > 0 && up[0] > 0 {
		offset = +up[0]
	}
	pc, _, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}

	f := runtime.FuncForPC(pc)
	if f == nil {
		panic("failed to get function name")
	}
	return strings.TrimPrefix(filepath.Ext(f.Name()), ".")
}

func FileLine(up ...int) string {
	offset := 1
	if len(up) > 0 && up[0] > 0 {
		offset += up[0]
	}
	pc, _, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}

	f := runtime.FuncForPC(pc)
	if f == nil {
		panic("failed to get function name")
	}
	file, line := f.FileLine(pc)
	return fmt.Sprintf("%s:%d", file, line)
}

func CallerFuncName(up ...int) string {
	offset := 2
	if len(up) > 0 && up[0] > 0 {
		offset += up[0]
	}
	pc, _, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}

	f := runtime.FuncForPC(pc)
	if f == nil {
		panic("failed to get function name")
	}
	return strings.TrimPrefix(filepath.Ext(f.Name()), ".")
}

func CallerFileLine(up ...int) string {
	offset := 2
	if len(up) > 0 && up[0] > 0 {
		offset += up[0]
	}
	pc, _, _, ok := runtime.Caller(offset)
	if !ok {
		panic("failed to get caller")
	}

	f := runtime.FuncForPC(pc)
	if f == nil {
		panic("failed to get function name")
	}
	file, line := f.FileLine(pc)
	return fmt.Sprintf("%s:%d", file, line)
}
