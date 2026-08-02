package main

import (
	"fmt"
	"os"
	_ "time/tzdata"

	"github.com/usenorn/norn/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
