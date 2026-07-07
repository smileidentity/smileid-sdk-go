package main

import (
	"context"
	"fmt"
	"os"

	"github.com/smileidentity/smileid-example-go/internal/example"
)

func main() {
	if err := example.Run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if example.IsUsageError(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}
