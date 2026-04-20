package domain

import (
	"strings"
	"testing"
)

// ---- hasRealValues ----

func TestHasRealValues_NoObservations(t *testing.T) {
	et := EndpointTemplate{}
	if hasRealValues(et) {
		t.Error("expected false for empty observations")
	}
}

func TestHasRealValues_OnlySwagger(t *testing.T) {
	et := EndpointTemplate{
		Observations: []ExampleURL{{Source: "swagger", URL: "https://example.com/v1/users/{id}"}},
	}
	if hasRealValues(et) {
		t.Error("expected false for swagger-only observations")
	}
}

func TestHasRealValues_Wayback(t *testing.T) {
	et := EndpointTemplate{
		Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
	}
	if !hasRealValues(et) {
		t.Error("expected true for wayback observation")
	}
}

func TestHasRealValues_Logs(t *testing.T) {
	et := EndpointTemplate{
		Observations: []ExampleURL{{Source: "logs", URL: "https://example.com/v1/users/123"}},
	}
	if !hasRealValues(et) {
		t.Error("expected true for logs observation")
	}
}

// ---- isGenericParamName ----

func TestIsGenericParamName(t *testing.T) {
	cases := []struct {
		name    string
		generic bool
	}{
		{"id", true},
		{"id2", true},
		{"id3", true},
		{"id10", true},
		{"uuid", true},
		{"uuid2", true},
		{"uuid3", true},
		{"userId", false},
		{"orderId", false},
		{"bookingId", false},
		{"id_user", false},
		{"myid", false},
		{"uuidStr", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGenericParamName(tc.name); got != tc.generic {
				t.Errorf("isGenericParamName(%q) = %v, want %v", tc.name, got, tc.generic)
			}
		})
	}
}

// ---- extractValuesFromObservations ----

func TestExtractValuesFromObservations_SingleParam(t *testing.T) {
	template := "/v1/users/{userId}"
	obs := []ExampleURL{
		{Source: "wayback", URL: "https://example.com/v1/users/123"},
		{Source: "logs", URL: "https://example.com/v1/users/456"},
	}
	vals := extractValuesFromObservations(template, obs)
	if len(vals["userId"]) != 2 {
		t.Errorf("expected 2 values for userId, got %v", vals["userId"])
	}
}

func TestExtractValuesFromObservations_MultipleParams(t *testing.T) {
	template := "/v1/org/{orgId}/item/{itemId}"
	obs := []ExampleURL{
		{Source: "wayback", URL: "https://example.com/v1/org/10/item/99"},
	}
	vals := extractValuesFromObservations(template, obs)
	if vals["orgId"][0] != "10" {
		t.Errorf("expected orgId=10, got %v", vals["orgId"])
	}
	if vals["itemId"][0] != "99" {
		t.Errorf("expected itemId=99, got %v", vals["itemId"])
	}
}

func TestExtractValuesFromObservations_SkipsSwagger(t *testing.T) {
	template := "/v1/users/{userId}"
	obs := []ExampleURL{
		{Source: "swagger", URL: "https://example.com/v1/users/ignored"},
	}
	vals := extractValuesFromObservations(template, obs)
	if len(vals) != 0 {
		t.Errorf("expected no values from swagger observation, got %v", vals)
	}
}

func TestExtractValuesFromObservations_Deduplicates(t *testing.T) {
	template := "/v1/users/{userId}"
	obs := []ExampleURL{
		{Source: "wayback", URL: "https://example.com/v1/users/123"},
		{Source: "logs", URL: "https://example.com/v1/users/123"},
	}
	vals := extractValuesFromObservations(template, obs)
	if len(vals["userId"]) != 1 {
		t.Errorf("expected 1 deduplicated value, got %v", vals["userId"])
	}
}

func TestExtractValuesFromObservations_SkipsMismatchedSegmentCount(t *testing.T) {
	template := "/v1/users/{userId}"
	obs := []ExampleURL{
		{Source: "wayback", URL: "https://example.com/v1/users/123/extra"},
	}
	vals := extractValuesFromObservations(template, obs)
	if len(vals) != 0 {
		t.Errorf("expected no values for mismatched segment count, got %v", vals)
	}
}

// ---- Part 1: inferFromParentEndpoints ----

func TestInferIDs_Part1_BasicParentChild(t *testing.T) {
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/something/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/something/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/something/{id}/blah",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	child := result[1]
	if len(child.Examples) == 0 {
		t.Fatal("expected inferred examples for child endpoint")
	}
	found := false
	for _, ex := range child.Examples {
		if ex.ParamName == "id" && ex.Value == "123" && ex.Source == "inferred" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected inferred id=123, got: %+v", child.Examples)
	}
}

func TestInferIDs_Part1_MismatchedParamNames_NoInference(t *testing.T) {
	// Spec: /a/{id} and /a/{something}/b — param names differ, should not match.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/a/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/a/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/a/{something}/b",
			Parameters:   []Parameter{{Name: "something", Type: "string", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 0 {
		t.Errorf("expected no inferred examples when param names differ, got: %+v", result[1].Examples)
	}
}

func TestInferIDs_Part1_SkipsChildWithRealValues(t *testing.T) {
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}/orders",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{{Source: "logs", URL: "https://example.com/v1/users/456/orders"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 0 {
		t.Errorf("expected no inferences for endpoint with real observations, got: %+v", result[1].Examples)
	}
}

func TestInferIDs_Part1_LongestParentWins(t *testing.T) {
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/111"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/{id}/users",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/222/users"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/{id}/users/detail",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	child := result[2]
	if len(child.Examples) == 0 {
		t.Fatal("expected inferred examples for child endpoint")
	}
	// Longest parent is /v1/{id}/users with id=222 (len 4 > len 3 for /v1/{id})
	found222 := false
	for _, ex := range child.Examples {
		if ex.ParamName == "id" && ex.Value == "222" {
			found222 = true
		}
	}
	if !found222 {
		t.Errorf("expected value 222 from longest parent, got: %+v", child.Examples)
	}
	// Should NOT contain 111 from the shorter parent
	for _, ex := range child.Examples {
		if ex.ParamName == "id" && ex.Value == "111" {
			t.Errorf("unexpected value 111 from shorter parent, got: %+v", child.Examples)
		}
	}
}

func TestInferIDs_Part1_MultipleValuesFromParent(t *testing.T) {
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{
				{Source: "wayback", URL: "https://example.com/v1/users/123"},
				{Source: "logs", URL: "https://example.com/v1/users/456"},
			},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}/profile",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 2 {
		t.Errorf("expected 2 inferred examples (one per observed value), got %d: %+v",
			len(result[1].Examples), result[1].Examples)
	}
}

func TestInferIDs_Part1_StaticSegmentMismatch_NoInference(t *testing.T) {
	// /v1/users/{id} is NOT a prefix of /v1/orders/{id}/detail because "users" != "orders"
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/orders/{id}/detail",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 0 {
		t.Errorf("expected no inferences for non-prefix parent, got: %+v", result[1].Examples)
	}
}

func TestInferIDs_Part1_MultipleChildParams(t *testing.T) {
	// Parent has {orgId}, child has {orgId}/{itemId}; only {orgId} should be inferred
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/org/{orgId}",
			Parameters:   []Parameter{{Name: "orgId", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/org/10"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/org/{orgId}/item/{itemId}",
			Parameters: []Parameter{
				{Name: "orgId", Type: "int", Source: "swagger"},
				{Name: "itemId", Type: "int", Source: "swagger"},
			},
		},
	}

	result := InferIDs(endpoints)

	child := result[1]
	foundOrgId := false
	for _, ex := range child.Examples {
		if ex.ParamName == "orgId" && ex.Value == "10" {
			foundOrgId = true
		}
		if ex.ParamName == "itemId" {
			t.Errorf("unexpected inferred value for itemId: %+v", ex)
		}
	}
	if !foundOrgId {
		t.Errorf("expected inferred orgId=10, got: %+v", child.Examples)
	}
}

// ---- Part 2: inferFromMatchingParamNames ----

func TestInferIDs_Part2_CrossEndpointNonGenericParam(t *testing.T) {
	// Spec example: route=/v1/users/{userId} with userId=123 → infer /v1/bookings/{userId}
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/bookings/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	bookings := result[1]
	found := false
	for _, ex := range bookings.Examples {
		if ex.ParamName == "userId" && ex.Value == "123" && ex.Source == "inferred" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected inferred userId=123 for /v1/bookings/{userId}, got: %+v", bookings.Examples)
	}
}

func TestInferIDs_Part2_GenericParamNotInferred(t *testing.T) {
	// Generic names (id, uuid, etc.) must not trigger cross-endpoint inference.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/bookings/{id}",
			Parameters:   []Parameter{{Name: "id", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 0 {
		t.Errorf("expected no inferences for generic param 'id', got: %+v", result[1].Examples)
	}
}

func TestInferIDs_Part2_SkipsEndpointWithRealValues(t *testing.T) {
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/bookings/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{{Source: "logs", URL: "https://example.com/v1/bookings/456"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 0 {
		t.Errorf("expected no inferences for endpoint with real values, got: %+v", result[1].Examples)
	}
}

func TestInferIDs_Part2_MultipleValuesAcrossEndpoints(t *testing.T) {
	// userId=123 and userId=456 observed in two different real endpoints → both inferred
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{
				{Source: "wayback", URL: "https://example.com/v1/users/123"},
				{Source: "logs", URL: "https://example.com/v1/users/456"},
			},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/bookings/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	if len(result[1].Examples) != 2 {
		t.Errorf("expected 2 inferred examples, got %d: %+v", len(result[1].Examples), result[1].Examples)
	}
}

// ---- Combined behaviour ----

func TestInferIDs_Part1AndPart2_NoDuplicates(t *testing.T) {
	// /v1/users/{userId} has userId=123 (Part 1 parent, Part 2 source).
	// /v1/users/{userId}/settings is a child of the above (Part 1 applies).
	// Part 2 would also match userId → 123. appendUniqueExamples should prevent duplicates.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "inferred"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}/settings",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		},
	}

	result := InferIDs(endpoints)

	settings := result[1]
	if len(settings.Examples) == 0 {
		t.Fatal("expected at least one inferred example")
	}
	count := 0
	for _, ex := range settings.Examples {
		if ex.ParamName == "userId" && ex.Value == "123" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 userId=123 example (no duplicates), got %d: %+v", count, settings.Examples)
	}
}

func TestInferIDs_DoesNotModifyInput(t *testing.T) {
	original := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/bookings/{userId}",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
		},
	}

	InferIDs(original)

	if len(original[1].Examples) != 0 {
		t.Error("InferIDs must not modify the input slice")
	}
}

func TestInferIDs_EmptyInput(t *testing.T) {
	result := InferIDs(nil)
	if result == nil || len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %v", result)
	}

	result = InferIDs([]EndpointTemplate{})
	if len(result) != 0 {
		t.Errorf("expected empty result for empty input, got %v", result)
	}
}

// ---- inferObservations (Part 3) ----

func TestInferIDs_Part3_SkipsUnresolvedPlaceholders(t *testing.T) {
	// An endpoint whose parameter has no example value should not get an inferred
	// observation URL containing a `{...}` placeholder.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{username}",
			Parameters:   []Parameter{{Name: "username", Type: "string", Source: "swagger"}},
			// No examples — username=? is unknown
		},
	}

	result := InferIDs(endpoints)

	for _, obs := range result[1].Observations {
		if strings.Contains(obs.URL, "{") {
			t.Errorf("inferred observation URL must not contain unresolved placeholders, got: %s", obs.URL)
		}
	}
	if len(result[1].Observations) != 0 {
		t.Errorf("expected no inferred observations when placeholder cannot be resolved, got: %+v", result[1].Observations)
	}
}

func TestInferIDs_Part3_AddsObservationWhenResolved(t *testing.T) {
	// An endpoint whose parameter can be resolved should get an inferred observation.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}",
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/users/123"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/users/{userId}/settings",
			Parameters:   []Parameter{{Name: "userId", Type: "int", Source: "swagger"}},
			// No real observations; userId will be inferred from parent
		},
	}

	result := InferIDs(endpoints)

	settings := result[1]
	found := false
	for _, obs := range settings.Observations {
		if obs.Source == "inferred" && obs.URL == "https://example.com/v1/users/123/settings" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected inferred observation https://example.com/v1/users/123/settings, got: %+v", settings.Observations)
	}
}

func TestInferIDs_Part3_SkipsPartiallyResolved(t *testing.T) {
	// An endpoint with two parameters where only one can be resolved must NOT get
	// an inferred observation URL that still contains an unsubstituted placeholder.
	endpoints := []EndpointTemplate{
		{
			Method:       "GET",
			PathTemplate: "/v1/org/{orgId}",
			Observations: []ExampleURL{{Source: "wayback", URL: "https://example.com/v1/org/10"}},
		},
		{
			Method:       "GET",
			PathTemplate: "/v1/org/{orgId}/item/{itemId}",
			Parameters: []Parameter{
				{Name: "orgId", Type: "int", Source: "swagger"},
				{Name: "itemId", Type: "int", Source: "swagger"},
			},
			// orgId is inferred from the parent endpoint; itemId has no known value
		},
	}

	result := InferIDs(endpoints)

	child := result[1]

	// orgId should be present in Examples (inference still runs)
	foundOrgId := false
	for _, ex := range child.Examples {
		if ex.ParamName == "orgId" {
			foundOrgId = true
		}
	}
	if !foundOrgId {
		t.Fatalf("expected orgId to be inferred in Examples, got: %+v", child.Examples)
	}

	// No inferred observation should be created because itemId is still unresolved.
	for _, obs := range child.Observations {
		if strings.Contains(obs.URL, "{") {
			t.Errorf("inferred observation URL must not contain unresolved placeholders, got: %s", obs.URL)
		}
	}
	if len(child.Observations) != 0 {
		t.Errorf("expected no inferred observations when not all placeholders can be resolved, got: %+v", child.Observations)
	}
}
