package main

import (
	"fmt"
	"os"

	"github.com/ynori7/draque/domain"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <log-file> <input-format-pattern>\n", os.Args[0])
		os.Exit(2)
	}

	endpoints, err := domain.ParseAccessLog(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "access log parse failed: %v\n", err)
		os.Exit(1)
	}

	for _, ep := range endpoints {
		prefix := ""
		if ep.Method != "" {
			prefix = ep.Method + " "
		}

		example := ""
		if len(ep.Observations) > 0 {
			example = " (" + ep.Observations[0].URL + ")"
		}

		fmt.Printf("%s%s%s\n", prefix, ep.PathTemplate, example)
	}
}
