package main

import (
	"os"

	"github.com/idahoakl/aws-vpc-exporter/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
