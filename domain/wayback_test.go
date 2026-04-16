package domain

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestWaybackFetcherFetchURLsFiltersAndDeduplicates(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()
		if query.Get("showNumPages") == "true" {
			http.Error(writer, "pagination unsupported", http.StatusBadRequest)
			return
		}

		if got := query.Get("url"); got != "example.com/api" {
			t.Fatalf("unexpected query target: %q", got)
		}

		if got := query.Get("matchType"); got != "domain" {
			t.Fatalf("unexpected matchType: %q", got)
		}

		if got := query.Get("fl"); got != "original,statuscode" {
			t.Fatalf("unexpected field list: %q", got)
		}

		writer.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(writer).Encode([][]string{
			{"original", "statuscode"},
			{"https://example.com", "200"},
			{"https://example.com/", "200"},
			{"ftp://example.com/api/users", "200"},
			{"https://example.com/api/users", "404"},
			{"https://example.com/static/app.js", "200"},
			{"https://EXAMPLE.com:443/api/users?b=2&a=1#fragment", "200"},
			{"https://example.com/api/users?a=1&b=2", "200"},
			{"https://example.com/api/users?a=99", "200"},
			{"http://example.com/api/widgets", "200"},
			{"https://example.com/apix", "200"},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	fetcher := WaybackFetcher{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	urls, err := fetcher.FetchURLs(context.Background(), "example.com", "/api")
	if err != nil {
		t.Fatalf("FetchURLs returned error: %v", err)
	}

	want := []string{
		"https://example.com/api/users?a=1&b=2",
		"http://example.com/api/widgets",
	}

	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("unexpected URLs\nwant: %#v\ngot:  %#v", want, urls)
	}
}

func TestWaybackFetcherFetchURLsUsesPagination(t *testing.T) {
	t.Helper()

	pageRequests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		query := request.URL.Query()

		if query.Get("showNumPages") == "true" {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("2\n"))
			return
		}

		pageRequests = append(pageRequests, query.Get("page"))
		writer.Header().Set("Content-Type", "application/json")

		switch query.Get("page") {
		case "0":
			_ = json.NewEncoder(writer).Encode([][]string{
				{"original", "statuscode"},
				{"https://example.com/api/one", "200"},
				{"https://example.com/assets/logo.svg", "200"},
			})
		case "1":
			_ = json.NewEncoder(writer).Encode([][]string{
				{"original", "statuscode"},
				{"https://example.com/api/two", "200"},
				{"https://example.com/api/one", "200"},
				{"https://example.com/api/two", "404"},
			})
		default:
			t.Fatalf("unexpected page request: %q", query.Get("page"))
		}
	}))
	defer server.Close()

	fetcher := WaybackFetcher{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		PageSize:   1,
	}

	urls, err := fetcher.FetchURLs(context.Background(), "example.com", "")
	if err != nil {
		t.Fatalf("FetchURLs returned error: %v", err)
	}

	wantURLs := []string{
		"https://example.com/api/one",
		"https://example.com/api/two",
	}
	if !reflect.DeepEqual(urls, wantURLs) {
		t.Fatalf("unexpected URLs\nwant: %#v\ngot:  %#v", wantURLs, urls)
	}

	wantPages := []string{"0", "1"}
	if !reflect.DeepEqual(pageRequests, wantPages) {
		t.Fatalf("unexpected page requests\nwant: %#v\ngot:  %#v", wantPages, pageRequests)
	}
}

func TestBuildWaybackQueryTarget(t *testing.T) {
	t.Helper()

	target, prefix, err := buildWaybackQueryTarget("https://Example.com:8443/root", "v1/")
	if err != nil {
		t.Fatalf("buildWaybackQueryTarget returned error: %v", err)
	}

	if target != "example.com:8443/v1" {
		t.Fatalf("unexpected target: %q", target)
	}

	if prefix != "/v1" {
		t.Fatalf("unexpected prefix: %q", prefix)
	}
}
