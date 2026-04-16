package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ynori7/draque/domain"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <domain> [path-prefix]\n", os.Args[0])
		os.Exit(2)
	}

	pathPrefix := ""
	if len(os.Args) == 3 {
		pathPrefix = os.Args[2]
	}

	endpoints, err := domain.FetchWaybackURLs(context.Background(), os.Args[1], pathPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wayback fetch failed: %v\n", err)
		os.Exit(1)
	}

	for _, ep := range endpoints {
		example := ""
		if len(ep.Observations) > 0 {
			example = " (" + ep.Observations[0].URL + ")"
		}
		fmt.Printf("%s%s\n", ep.PathTemplate, example)
	}
}
