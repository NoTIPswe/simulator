package main

import (
	"os"

	"github.com/NoTIPswe/notip-simulator-cli/cmd"
)

var execute = cmd.Execute
var osExit = os.Exit

func main() {
	if err := execute(); err != nil {
		osExit(1)
	}
}
