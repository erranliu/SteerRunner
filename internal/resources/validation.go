package resources

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

type Issue struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type ValidationResult struct {
	Kind           string  `json:"kind"`
	Valid          bool    `json:"valid"`
	Issues         []Issue `json:"issues"`
	Version        string  `json:"version"`
	ResourceDigest string  `json:"resource_digest"`
}

func ValidationKind(value string) bool {
	return value == "run-configuration" || value == "run"
}

// Validate calls the unchanged gate first. Schema diagnostics never determine
// acceptance or replace its fault/priority, including legacy extension inputs.
func Validate(value string, data []byte) (ValidationResult, *contracts.Fault) {
	result := ValidationResult{Kind: value, Issues: []Issue{}, Version: Version, ResourceDigest: Digest()}
	var fault *contracts.Fault
	switch value {
	case "run-configuration":
		fault = contracts.ValidateRunConfiguration(data)
	case "run":
		fault = contracts.ValidateRun(data)
	default:
		return result, contracts.Fail(2, "invalid_arguments", "Unknown validation kind.")
	}
	result.Valid = fault == nil
	if diagnosticSchema(value) == nil {
		return result, contracts.Fail(1, "internal_error", "The embedded validation contract is unavailable.")
	}
	if fault == nil {
		return result, nil
	}
	result.Issues = schemaIssues(value, data)
	if len(result.Issues) == 0 {
		result.Issues = []Issue{{"", fault.Detail.Code, fault.Detail.Message}}
	}
	return result, fault
}

var validationSchemas struct {
	sync.Once
	values map[string]*jsonschema.Schema
}

func diagnosticSchema(value string) *jsonschema.Schema {
	validationSchemas.Do(func() {
		validationSchemas.values = map[string]*jsonschema.Schema{}
		var values map[string]json.RawMessage
		if json.Unmarshal(ConfigurationSchemas(), &values) != nil {
			return
		}
		for name, raw := range map[string]json.RawMessage{"run-configuration": values["run_configuration"], "run": values["run_input"]} {
			doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
			if err != nil {
				continue
			}
			compiler := jsonschema.NewCompiler()
			url := "https://runner.invalid/" + name
			if compiler.AddResource(url, doc) != nil {
				continue
			}
			schema, err := compiler.Compile(url)
			if err == nil {
				validationSchemas.values[name] = schema
			}
		}
	})
	return validationSchemas.values[value]
}
func schemaIssues(value string, data []byte) []Issue {
	schema := diagnosticSchema(value)
	if schema == nil {
		return nil
	}
	var input any
	if json.Unmarshal(data, &input) != nil {
		return []Issue{{"", "invalid_json", "Expected one valid JSON object."}}
	}
	var validation *jsonschema.ValidationError
	if !errors.As(schema.Validate(input), &validation) {
		return nil
	}
	issues := []Issue{}
	seen := map[string]bool{}
	var collect func(*jsonschema.ValidationError)
	appendIssue := func(path, code, message string) {
		key := path + "\x00" + code
		if !seen[key] && len(issues) < 32 {
			seen[key] = true
			issues = append(issues, Issue{path, code, message})
		}
	}
	collect = func(err *jsonschema.ValidationError) {
		path := ""
		for _, part := range err.InstanceLocation {
			path += "/" + strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
		}
		switch detail := err.ErrorKind.(type) {
		case *kind.Required:
			for _, field := range detail.Missing {
				appendIssue(path+"/"+strings.ReplaceAll(strings.ReplaceAll(field, "~", "~0"), "/", "~1"), "required", "A required field is missing.")
			}
		default:
			if len(err.Causes) == 0 {
				code := "invalid_value"
				switch err.ErrorKind.(type) {
				case *kind.Type:
					code = "invalid_type"
				case *kind.AdditionalProperties:
					code = "unknown_field"
				}
				appendIssue(path, code, "The value does not satisfy the published field contract.")
			}
		}
		for _, cause := range err.Causes {
			collect(cause)
		}
	}
	collect(validation)
	return issues
}
