package main

import (
	"runtime"

	"github.com/iotexproject/iotex-core/tools/actioninjector.v2/internal/cmd"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	cmd.Execute()
}
