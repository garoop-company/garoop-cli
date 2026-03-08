package main

import (
	"fmt"
	"os"

	"github.com/yamashitadaiki/garoop-cli/cmd"
)

func main() {
	if err := cmd.ExecuteWithProfile(cmd.ProfileGaroop); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
