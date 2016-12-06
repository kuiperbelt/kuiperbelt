// +build debug

package main

import (
	_ "net/http/pprof"
	"runtime"
)

func init() {
	runtime.MemProfileRate = 1024
}
