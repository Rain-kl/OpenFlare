// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Command generate-automation-catalog derives automation contracts from Swagger.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
)

const (
	outputPath   = "internal/automationcontract/catalog.json"
	catalogMode  = 0o600
	locationPath = "path"
)

type document struct {
	Paths       map[string]map[string]swaggerOperation `json:"paths"`
	Definitions map[string]map[string]any              `json:"definitions"`
}

type swaggerOperation struct {
	ID          string                     `json:"operationId"`
	Description string                     `json:"description"`
	Parameters  []map[string]any           `json:"parameters"`
	Responses   map[string]swaggerResponse `json:"responses"`
	Automation  *automationOptions         `json:"x-automation"`
}

type swaggerResponse struct {
	Schema map[string]any `json:"schema"`
}

type automationOptions struct {
	CLI        string                    `json:"cli"`
	Scope      string                    `json:"scope"`
	Requires   []string                  `json:"requires"`
	Effect     string                    `json:"effect"`
	Parameters map[string]map[string]any `json:"parameters"`
}

func main() {
	check := flag.Bool("check", false, "Check that the generated file is up to date")
	flag.Parse()
	if err := run(*check); err != nil {
		log.Fatal(err)
	}
}

func run(check bool) error {
	source, err := os.ReadFile("api/swagger/swagger.json")
	if err != nil {
		return err
	}
	var spec document
	if err := json.Unmarshal(source, &spec); err != nil {
		return err
	}
	operations, err := collect(spec)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(operations, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if !check {
		return os.WriteFile(outputPath, encoded, catalogMode)
	}
	current, err := os.ReadFile(outputPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(encoded, current) {
		return errors.New("automation catalog is stale: run make swagger")
	}
	return nil
}

func collect(spec document) ([]automationcontract.Operation, error) {
	operations := make([]automationcontract.Operation, 0)
	ids, commands := make(map[string]bool), make(map[string]bool)
	for path, methods := range spec.Paths {
		for method, source := range methods {
			if source.Automation == nil {
				continue
			}
			operation, err := convert(spec, path, method, source)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			if ids[operation.ID] || commands[operation.CLI] {
				return nil, fmt.Errorf("duplicate operation ID or command: %s %s", operation.ID, operation.CLI)
			}
			ids[operation.ID], commands[operation.CLI] = true, true
			operations = append(operations, operation)
		}
	}
	slices.SortFunc(operations, func(left, right automationcontract.Operation) int {
		return strings.Compare(left.ID, right.ID)
	})
	return operations, nil
}

var (
	operationID   = regexp.MustCompile(`^openflare_[a-z0-9_]+$`)
	commandPath   = regexp.MustCompile(`^[a-z][a-z0-9-]*( [a-z][a-z0-9-]*)+$`)
	scopeName     = regexp.MustCompile(`^[a-z][a-z0-9-]*:(read|write)$`)
	parameterName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	pathParam     = regexp.MustCompile(`\{([^{}]+)\}`)
)

func convert(spec document, path, method string, source swaggerOperation) (automationcontract.Operation, error) {
	opts := source.Automation
	read := method == "get" && opts.Effect == automationcontract.EffectRead
	write := (method == "post" || method == "patch" || method == "delete") && opts.Effect == automationcontract.EffectWrite
	if (!read && !write) || !strings.HasPrefix(path, "/api"+automationcontract.APISuffix+"/") {
		return automationcontract.Operation{}, errors.New("expected a read GET or write POST/PATCH/DELETE automation operation")
	}
	if !operationID.MatchString(source.ID) || !commandPath.MatchString(opts.CLI) ||
		!scopeName.MatchString(opts.Scope) || !strings.HasSuffix(opts.Scope, ":"+opts.Effect) || strings.TrimSpace(source.Description) == "" {
		return automationcontract.Operation{}, errors.New("invalid operation ID, CLI command, scope or description")
	}
	for i, scope := range opts.Requires {
		if !scopeName.MatchString(scope) || scope == opts.Scope || slices.Contains(opts.Requires[:i], scope) {
			return automationcontract.Operation{}, fmt.Errorf("invalid additional scope %q", scope)
		}
	}
	params, err := convertParameters(spec, path, source)
	if err != nil {
		return automationcontract.Operation{}, err
	}
	output, err := outputSchema(spec, source.Responses["200"].Schema)
	if err != nil {
		return automationcontract.Operation{}, err
	}
	return automationcontract.Operation{
		ID: source.ID, CLI: opts.CLI, Method: strings.ToUpper(method), Path: strings.TrimPrefix(path, "/api"+automationcontract.APISuffix),
		Scope: opts.Scope, Requires: opts.Requires, Effect: opts.Effect, Description: source.Description, Parameters: params, Output: output,
	}, nil
}

func convertParameters(spec document, path string, source swaggerOperation) ([]automationcontract.Parameter, error) {
	params := make([]automationcontract.Parameter, 0, len(source.Parameters))
	names := make(map[string]string)
	entries, err := flattenParameters(spec, source)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		location, _ := entry["in"].(string)
		description, _ := entry["description"].(string)
		required, _ := entry["required"].(bool)
		if location == automationcontract.LocationBody {
			required, _ = entry["x-required-field"].(bool)
		}
		if !validParameter(name, location, description) || names[name] != "" {
			return nil, fmt.Errorf("invalid automation parameter %q", name)
		}
		if location == locationPath && (!required || !strings.Contains(path, "{"+name+"}")) {
			return nil, fmt.Errorf("path parameter %s must match a placeholder and be required", name)
		}
		names[name] = location
		schema, err := parameterSchema(entry, source.Automation.Parameters[name])
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %w", name, err)
		}
		params = append(params, automationcontract.Parameter{Name: name, In: location, Required: required, Description: description, Schema: schema})
	}
	for _, match := range pathParam.FindAllStringSubmatch(path, -1) {
		if names[match[1]] != locationPath {
			return nil, fmt.Errorf("undeclared path parameter %s", match[1])
		}
	}
	for name := range source.Automation.Parameters {
		if names[name] == "" {
			return nil, fmt.Errorf("override for undeclared parameter %s", name)
		}
	}
	return params, nil
}

func validParameter(name, location, description string) bool {
	if !parameterName.MatchString(name) || name == "profile" || name == "output" || name == "help" || strings.TrimSpace(description) == "" {
		return false
	}
	return location == "query" || location == locationPath || location == automationcontract.LocationBody
}

func parameterSchema(entry, overrides map[string]any) (map[string]any, error) {
	schema := map[string]any{"type": entry["type"]}
	switch schema["type"] {
	case automationcontract.SchemaArray:
		items, _ := entry["items"].(map[string]any)
		if items == nil || entry["in"] != automationcontract.LocationBody {
			return nil, errors.New("arrays must be JSON body fields with explicit items")
		}
		schema["items"] = closeInputObjects(items)
		if items["type"] == automationcontract.SchemaString {
			schema["items"] = map[string]any{"type": automationcontract.SchemaString, "minLength": 1, "maxLength": 2048}
		}
		schema["minItems"], schema["maxItems"] = 1, 32
	case automationcontract.SchemaObject:
		if entry["in"] != automationcontract.LocationBody {
			return nil, errors.New("objects must be JSON body fields")
		}
		schema = closeInputObjects(entry)
		for _, key := range []string{"name", "in", "description", "x-required-field"} {
			delete(schema, key)
		}
		if _, ok := schema["required"].(bool); ok {
			delete(schema, "required")
		}
	case automationcontract.SchemaInteger, automationcontract.SchemaString, automationcontract.SchemaBoolean:
	default:
		return nil, errors.New("unsupported parameter type")
	}
	for _, key := range []string{"enum", "default", "minimum", "maximum", "minLength", "maxLength", "pattern", "minItems", "maxItems"} {
		if value, exists := entry[key]; exists {
			schema[key] = value
		}
	}
	if err := applyParameterOverrides(schema, overrides); err != nil {
		return nil, err
	}
	return schema, nil
}

func applyParameterOverrides(schema, overrides map[string]any) error {
	for key, value := range overrides {
		if size, ok := value.(float64); ok && schema["type"] == automationcontract.SchemaArray && (key == "minItems" || key == "maxItems") && size >= 0 && size == float64(int(size)) {
			schema[key] = size
			continue
		}
		pattern, ok := value.(string)
		if key != "pattern" || !ok || schema["type"] != automationcontract.SchemaString {
			return fmt.Errorf("unsupported override %s", key)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return err
		}
		schema[key] = pattern
	}
	return nil
}

func closeInputObjects(schema map[string]any) map[string]any {
	result := maps.Clone(schema)
	if properties, ok := schema["properties"].(map[string]any); ok {
		children := make(map[string]any, len(properties))
		for key, value := range properties {
			children[key] = closeInputObjects(value.(map[string]any))
		}
		result["properties"] = children
		result["additionalProperties"] = false
	}
	if items, ok := schema["items"].(map[string]any); ok {
		result["items"] = closeInputObjects(items)
	}
	return result
}

func flattenParameters(spec document, source swaggerOperation) ([]map[string]any, error) {
	var entries []map[string]any
	hasBody := false
	for _, entry := range source.Parameters {
		if entry["in"] != automationcontract.LocationBody {
			entries = append(entries, entry)
			continue
		}
		if hasBody || source.Automation.Effect != automationcontract.EffectWrite || entry["required"] != true {
			return nil, errors.New("expected one required JSON body on a write operation")
		}
		hasBody = true
		schema, _ := entry["schema"].(map[string]any)
		resolved, err := resolveSchema(spec, schema, make(map[string]bool))
		if err != nil {
			return nil, err
		}
		properties, _ := resolved["properties"].(map[string]any)
		if resolved["type"] != "object" || len(properties) == 0 {
			return nil, errors.New("expected explicit JSON body fields")
		}
		required, _ := resolved["required"].([]any)
		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			field, _ := properties[name].(map[string]any)
			field["name"], field["in"], field["x-required-field"] = name, automationcontract.LocationBody, slices.Contains(required, any(name))
			entries = append(entries, field)
		}
	}
	return entries, nil
}

func outputSchema(spec document, envelope map[string]any) (map[string]any, error) {
	parts, _ := envelope["allOf"].([]any)
	for _, part := range parts {
		fields, _ := part.(map[string]any)
		properties, _ := fields["properties"].(map[string]any)
		data, _ := properties["data"].(map[string]any)
		if data == nil {
			continue
		}
		return resolveSchema(spec, data, make(map[string]bool))
	}
	return nil, errors.New("expected an explicit success data DTO")
}

func resolveSchema(spec document, input map[string]any, parents map[string]bool) (map[string]any, error) {
	// Swaggo wraps a reference in a single allOf entry when the field has a description.
	if parts, ok := input["allOf"].([]any); ok && len(parts) == 1 {
		if ref, ok := parts[0].(map[string]any); ok && ref["$ref"] != nil {
			resolved, err := resolveSchema(spec, ref, parents)
			if err != nil {
				return nil, err
			}
			siblings := maps.Clone(input)
			delete(siblings, "allOf")
			maps.Copy(resolved, siblings)
			return resolved, nil
		}
	}
	if ref, ok := input["$ref"].(string); ok {
		key := strings.TrimPrefix(ref, "#/definitions/")
		definition := spec.Definitions[key]
		if definition == nil || parents[key] {
			return nil, fmt.Errorf("unknown or recursive output reference %s", ref)
		}
		parents[key] = true
		resolved, err := resolveSchema(spec, definition, parents)
		delete(parents, key)
		return resolved, err
	}
	resolved := make(map[string]any, len(input))
	for key, value := range input {
		next, err := resolveSchemaValue(spec, key, value, parents)
		if err != nil {
			return nil, err
		}
		resolved[key] = next
	}
	return resolved, nil
}

func resolveSchemaValue(spec document, key string, value any, parents map[string]bool) (any, error) {
	object, isObject := value.(map[string]any)
	if !isObject {
		return value, nil
	}
	if key != "properties" {
		return resolveSchema(spec, object, parents)
	}
	properties := make(map[string]any, len(object))
	for name, raw := range object {
		schema, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid property schema for %s", name)
		}
		resolved, err := resolveSchema(spec, schema, parents)
		if err != nil {
			return nil, err
		}
		properties[name] = resolved
	}
	return properties, nil
}
