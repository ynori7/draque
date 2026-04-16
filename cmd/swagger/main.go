package main

import (
	"fmt"
	"os"

	"github.com/ynori7/draque/domain"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <spec-file>\n", os.Args[0])
		os.Exit(2)
	}

	endpoints, err := domain.ParseSwaggerSpec(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, ep := range endpoints {
		fmt.Printf("%s %s\n", ep.Method, ep.PathTemplate)
	}
}

