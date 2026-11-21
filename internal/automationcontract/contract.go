// Package automationcontract defines the shared REST, CLI, and MCP operation contract.
package automationcontract

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Transport paths, headers, and schema types shared by the automation clients and server.
const (
	APISuffix      = "/v1/automation"
	MCPPath        = "/mcp"
	OAuthPath      = "/oauth/automation"
	InstanceHeader = "X-OpenFlare-Instance-Id"
	ContractHeader = "X-OpenFlare-Contract"
	EffectRead     = "read"
	EffectWrite    = "write"
	SchemaArray    = "array"
	SchemaBoolean  = "boolean"
	SchemaInteger  = "integer"
	SchemaString   = "string"
	LocationBody   = "body"
)

//go:embed catalog.json
var catalogJSON []byte

// Operation describes one public action and its transport-independent contract.
type Operation struct {
	ID          string         `json:"id"`
	CLI         string         `json:"cli"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Scope       string         `json:"scope"`
	Effect      string         `json:"effect"`
	Description string         `json:"description"`
	Parameters  []Parameter    `json:"parameters"`
	Output      map[string]any `json:"output"`
}

// Parameter describes one named input and its REST location.
type Parameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required,omitempty"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

// Operations loads the generated catalog.
func Operations() []Operation {
	var operations []Operation
	if err := json.Unmarshal(catalogJSON, &operations); err != nil {
		panic(err)
	}
	return operations
}

// InputSchema builds the closed object schema used by MCP.
func (operation Operation) InputSchema() map[string]any {
	properties := make(map[string]any, len(operation.Parameters))
	required := make([]string, 0, len(operation.Parameters))
	for _, param := range operation.Parameters {
		schema := make(map[string]any, len(param.Schema)+1)
		for key, value := range param.Schema {
			schema[key] = value
		}
		schema["description"] = param.Description
		properties[param.Name] = schema
		if param.Required {
			required = append(required, param.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) != 0 {
		schema["required"] = required
	}
	return schema
}

// Validate rejects arguments outside the operation's contract.
func (operation Operation) Validate(input map[string]string) error {
	presentCount := 0
	for _, param := range operation.Parameters {
		value, present := input[param.Name]
		if !present {
			if param.Required {
				return fmt.Errorf("%s is required", param.Name)
			}
			continue
		}
		if err := param.validate(value); err != nil {
			return fmt.Errorf("%s: %w", param.Name, err)
		}
		presentCount++
	}
	if presentCount != len(input) {
		return errors.New("request contains an unknown argument")
	}
	return nil
}

// Revision hashes the operation contract for write compatibility checks.
func (operation Operation) Revision() string {
	encoded, err := json.Marshal(operation)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded))
}

func (param Parameter) validate(value string) error {
	switch param.Schema["type"] {
	case SchemaArray:
		return validateArray(value)
	case SchemaBoolean:
		if value != "true" && value != "false" {
			return errors.New("must be true or false")
		}
	case SchemaInteger:
		return param.validateInteger(value)
	}
	if enum, ok := param.Schema["enum"].([]any); ok {
		for _, allowed := range enum {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("must be one of %v", enum)
	}
	if limit, ok := param.Schema["maxLength"].(float64); ok && len(value) > int(limit) {
		return fmt.Errorf("must be at most %d bytes", int(limit))
	}
	if limit, ok := param.Schema["minLength"].(float64); ok && len(value) < int(limit) {
		return fmt.Errorf("must contain at least %d bytes", int(limit))
	}
	if pattern, ok := param.Schema["pattern"].(string); ok {
		match, err := regexp.MatchString(pattern, value)
		if err != nil || !match {
			return fmt.Errorf("must match %s", pattern)
		}
	}
	return nil
}

func validateArray(value string) error {
	var items []*string
	if err := json.Unmarshal([]byte(value), &items); err != nil || items == nil {
		return errors.New("must be a JSON array of strings")
	}
	if len(items) == 0 || len(items) > 32 {
		return errors.New("must contain 1 to 32 items")
	}
	for _, item := range items {
		if item == nil || *item == "" || len(*item) > 2048 {
			return errors.New("items must contain 1 to 2048 bytes")
		}
	}
	return nil
}

func (param Parameter) validateInteger(value string) error {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return errors.New("must be an integer")
	}
	if minimum, ok := param.Schema["minimum"].(float64); ok && float64(number) < minimum {
		return fmt.Errorf("must be at least %v", minimum)
	}
	if maximum, ok := param.Schema["maximum"].(float64); ok && float64(number) > maximum {
		return fmt.Errorf("must be at most %v", maximum)
	}
	return nil
}

// PathAndQuery validates the arguments and assembles the REST target.
func (operation Operation) PathAndQuery(input map[string]string) (string, error) {
	if err := operation.Validate(input); err != nil {
		return "", err
	}
	path, query := operation.Path, make(url.Values)
	for _, param := range operation.Parameters {
		if value, ok := input[param.Name]; ok {
			switch param.In {
			case "path":
				path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(value))
			case "query":
				query.Set(param.Name, value)
			}
		}
	}
	if len(query) != 0 {
		path += "?" + query.Encode()
	}
	return path, nil
}

// JSONBody encodes the body fields from validated CLI arguments.
func (operation Operation) JSONBody(input map[string]string) (json.RawMessage, error) {
	body := make(map[string]any)
	for _, param := range operation.Parameters {
		value, present := input[param.Name]
		if param.In != LocationBody || !present {
			continue
		}
		switch param.Schema["type"] {
		case SchemaInteger:
			number, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, err
			}
			body[param.Name] = number
		case SchemaBoolean, SchemaArray:
			body[param.Name] = json.RawMessage(value)
		default:
			body[param.Name] = value
		}
	}
	return json.Marshal(body)
}

// JSONInput decodes a closed object without accepting duplicate keys or coerced types.
func (operation Operation) JSONInput(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || (len(operation.Parameters) == 0 && bytes.Equal(bytes.TrimSpace(raw), []byte("null"))) {
		raw = json.RawMessage("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("arguments must be a JSON object")
	}
	input := make(map[string]string)
	parameters := make(map[string]Parameter, len(operation.Parameters))
	for _, param := range operation.Parameters {
		parameters[param.Name] = param
	}
	for decoder.More() {
		token, err = decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return nil, errors.New("invalid argument name")
		}
		param, ok := parameters[name]
		if !ok {
			return nil, fmt.Errorf("unknown argument %q", name)
		}
		if _, duplicate := input[name]; duplicate {
			return nil, fmt.Errorf("duplicate argument %q", name)
		}
		value, err := decodeArgument(decoder, param)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		input[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after arguments")
	}
	return input, nil
}

func decodeArgument(decoder *json.Decoder, param Parameter) (string, error) {
	if param.Schema["type"] == SchemaBoolean {
		var value *bool
		if err := decoder.Decode(&value); err != nil || value == nil {
			return "", errors.New("must be a boolean")
		}
		return strconv.FormatBool(*value), nil
	}
	if param.Schema["type"] == SchemaArray {
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return "", errors.New("must be an array")
		}
		return string(value), nil
	}
	if param.Schema["type"] == SchemaInteger {
		var number *int64
		if err := decoder.Decode(&number); err != nil || number == nil {
			return "", errors.New("must be an integer")
		}
		return strconv.FormatInt(*number, 10), nil
	}
	var value *string
	if err := decoder.Decode(&value); err != nil || value == nil {
		return "", errors.New("must be a string")
	}
	return *value, nil
}

// HTTPOrigin normalizes an HTTP or HTTPS origin and rejects paths and credentials.
func HTTPOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Opaque != "" ||
		(parsed.EscapedPath() != "" && parsed.EscapedPath() != "/") {
		return "", errors.New("expected an HTTP or HTTPS origin without a path, query, or credentials")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.Host = strings.ToLower(parsed.Host)
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":"+port)
	}
	return parsed.String(), nil
}
