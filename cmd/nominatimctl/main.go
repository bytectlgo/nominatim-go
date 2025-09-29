package main

import (
	"os"

	"nominatim-go/cmd/nominatimctl/tool/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
