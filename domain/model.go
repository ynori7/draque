package domain

// EndpointTemplate represents a normalized endpoint and all known evidence.
type EndpointTemplate struct {
	Method       string             `json:"method"`
	PathTemplate string             `json:"path_template"`
	Parameters   []Parameter        `json:"parameters"`   // ordered list of parameters appearing in the path template
	Examples     []ExampleParameter `json:"examples"`     // observed parameter values that contributed to this template
	Observations []ExampleURL       `json:"observations"` // observed URLs that contributed to this template
	Count        int                `json:"count"`
}

// Parameter describes a named endpoint parameter and where its metadata came from.
type Parameter struct {
	Name   string `json:"name"`
	Type   string `json:"type"`   // "int", "uuid", "string", "unknown"
	Source string `json:"source"` // "inferred", "swagger"
}

// ExampleParameter stores an observed value for a parameter.
type ExampleParameter struct {
	ParamName string `json:"param_name"`
	Value     string `json:"value"`
	Source    string `json:"source"` // "wayback", "logs", "swagger"
}

// ExampleURL captures a normalized source URL that contributed to an endpoint template.
type ExampleURL struct {
	Source     string `json:"source"`
	URL        string `json:"url"`         // normalized URL (not raw)
	StatusCode int    `json:"status_code"` // HTTP status code; 0 means unknown
}

// NormalizedPath is the output of URL normalization: a path template with inferred parameters.
// The normalized full URL is carried separately for observation storage.
type NormalizedPath struct {
	Template   string      `json:"template"`
	Parameters []Parameter `json:"parameters"`
}

// endpointObservation is an internal model to capture a single observed URL, HTTP method, 
// and status code that contributed to an endpoint template.
type endpointObservation struct {
	URL        string
	Method     string
	StatusCode int // 0 means unknown
}
