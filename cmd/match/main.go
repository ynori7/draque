package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ynori7/draque/domain"
)

func main() {
	swaggerFile := flag.String("swagger", "", "path to OpenAPI/Swagger spec file (.json, .yaml, .yml)")
	accesslogFile := flag.String("accesslog", "", "path to access log file")
	logPattern := flag.String("log-pattern", "", "access log format pattern (required with -accesslog)")
	waybackDomain := flag.String("wayback", "", "domain to fetch from the Wayback Machine CDX API")
	pathPrefix := flag.String("path-prefix", "", "path prefix filter for Wayback Machine results (optional)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "At least one of -swagger, -accesslog, or -wayback must be specified.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *swaggerFile == "" && *accesslogFile == "" && *waybackDomain == "" {
		flag.Usage()
		os.Exit(2)
	}

	if *accesslogFile != "" && *logPattern == "" {
		fmt.Fprintf(os.Stderr, "error: -log-pattern is required when -accesslog is specified\n")
		os.Exit(2)
	}

	var sources [][]domain.EndpointTemplate

	if *swaggerFile != "" {
		endpoints, err := domain.ParseSwaggerSpec(*swaggerFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "swagger parse failed: %v\n", err)
			os.Exit(1)
		}
		sources = append(sources, endpoints)
	}

	if *accesslogFile != "" {
		endpoints, err := domain.ParseAccessLog(*accesslogFile, *logPattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "access log parse failed: %v\n", err)
			os.Exit(1)
		}
		sources = append(sources, endpoints)
	}

	if *waybackDomain != "" {
		endpoints, err := domain.FetchWaybackURLs(context.Background(), *waybackDomain, *pathPrefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wayback fetch failed: %v\n", err)
			os.Exit(1)
		}
		sources = append(sources, endpoints)
	}

	merged := domain.MatchTemplates(sources...)

	for _, ep := range merged {
		example := ""
		if len(ep.Observations) > 0 {
			example = " (" + ep.Observations[0].URL + ")"
		}
		fmt.Printf("%s %s (count: %d)%s\n", ep.Method, ep.PathTemplate, ep.Count, example)
	}
}
