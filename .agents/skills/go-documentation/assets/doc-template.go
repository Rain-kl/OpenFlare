// Package example demonstrates proper Go documentation conventions.
//
// This package shows how to write doc comments for packages, types,
// functions, methods, and constants following Google Go Style Guide
// conventions.
//
// # Getting Started
//
// Create a new Widget with [NewWidget]:
//
//	w := example.NewWidget("name")
//	defer w.Close()
package example

import "errors"

// ErrNotFound is returned when a requested item does not exist.
var ErrNotFound = errors.New("example: not found")

// MaxRetries is the default number of retry attempts.
const MaxRetries = 3

// Widget processes items with configurable options.
//
// A zero-value Widget is not valid; use [NewWidget] to create one.
// Widget is safe for concurrent use.
//
// # Cleanup
//
// Call [Widget.Close] when done to release resources.
type Widget struct {
	name string
}

// NewWidget creates a Widget with the given name.
//
// Name must be non-empty; NewWidget panics otherwise.
func NewWidget(name string) *Widget {
	if name == "" {
		panic("example: name must be non-empty")
	}
	return &Widget{name: name}
}

// Process handles the given input and returns the result.
//
// Process returns [ErrNotFound] if the input references
// a missing item.
func (w *Widget) Process(input string) (string, error) {
	return input, nil
}

// Close releases resources held by the Widget.
func (w *Widget) Close() error {
	return nil
}

// Deprecated: Use [NewWidget] with functional options instead.
func NewWidgetLegacy(name string) *Widget {
	return NewWidget(name)
}
