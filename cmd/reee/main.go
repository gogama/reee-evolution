package main

import (
	"fmt"
	"os"

	"github.com/gogama/reee-evolution/cmd/reee/reeecmd"
)

func main() {
	err := reeecmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
