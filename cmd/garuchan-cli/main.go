package main

import (
	"fmt"
	"os"

	"garoop-cli/cmd"
)

func main() {
	if err := cmd.ExecuteWithProfile(cmd.ProfileGaruchan); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
