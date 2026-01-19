//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func configureDaemonProc(cmd *exec.Cmd) {
	// Windows doesn't use Setsid.
	// By default, started processes can be independent enough for our simple use case.
}

func restartSelf() error {
	fmt.Println("Please restart Neona to use the new version.")
	os.Exit(0)
	return nil
}
