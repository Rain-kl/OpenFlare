// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"Wavelet/core/contracts"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// TaskDefinition holds the definition and runtime options for an asynchronous background task.
type TaskDefinition struct {
	Pattern      string
	Type         string
	Name         string
	DisplayName  string
	Description  string
	Category     string
	SupportsTime bool
	Retryable    bool
	Queue        string
	Params       []contracts.TaskParamDTO
	Handler      any
	Concurrency  int
	Retry        int
	Timeout      time.Duration
	Metadata     map[string]any
}

// TaskOption configures a TaskDefinition.
type TaskOption func(*TaskDefinition)

// WithTaskType sets the admin task type identifier.
func WithTaskType(taskType string) TaskOption {
	return func(td *TaskDefinition) {
		td.Type = taskType
	}
}

// WithTaskName sets the task human-readable display name.
func WithTaskName(name string) TaskOption {
	return func(td *TaskDefinition) {
		td.Name = name
		if td.DisplayName == "" {
			td.DisplayName = name
		}
	}
}

// WithTaskDisplayName sets the task display name.
func WithTaskDisplayName(displayName string) TaskOption {
	return func(td *TaskDefinition) {
		td.DisplayName = displayName
		if td.Name == "" {
			td.Name = displayName
		}
	}
}

// WithTaskDescription sets the task description.
func WithTaskDescription(desc string) TaskOption {
	return func(td *TaskDefinition) {
		td.Description = desc
	}
}

// WithTaskCategory sets the task category grouping.
func WithTaskCategory(category string) TaskOption {
	return func(td *TaskDefinition) {
		td.Category = category
	}
}

// WithTaskSupportsTime sets whether the task supports time range filtering.
func WithTaskSupportsTime(supports bool) TaskOption {
	return func(td *TaskDefinition) {
		td.SupportsTime = supports
	}
}

// WithTaskQueue sets the task queue.
func WithTaskQueue(queue string) TaskOption {
	return func(td *TaskDefinition) {
		td.Queue = queue
	}
}

// WithTaskRetryable sets whether the task is retryable.
func WithTaskRetryable(retryable bool) TaskOption {
	return func(td *TaskDefinition) {
		td.Retryable = retryable
	}
}

// WithTaskParams sets the task parameter definitions.
func WithTaskParams(params ...contracts.TaskParamDTO) TaskOption {
	return func(td *TaskDefinition) {
		td.Params = append(td.Params, params...)
	}
}

// WithTaskMeta sets all task metadata from a TaskMetaDTO.
func WithTaskMeta(meta contracts.TaskMetaDTO) TaskOption {
	return func(td *TaskDefinition) {
		if meta.Type != "" {
			td.Type = meta.Type
		}
		if meta.Name != "" {
			td.Name = meta.Name
		}
		if meta.DisplayName != "" {
			td.DisplayName = meta.DisplayName
		}
		if meta.Description != "" {
			td.Description = meta.Description
		}
		if meta.Category != "" {
			td.Category = meta.Category
		}
		td.SupportsTime = meta.SupportsTime
		if meta.Queue != "" {
			td.Queue = meta.Queue
		}
		td.Retryable = meta.Retryable
		if meta.MaxRetry > 0 {
			td.Retry = meta.MaxRetry
		}
		if meta.Timeout > 0 {
			td.Timeout = meta.Timeout
		}
		if len(meta.Params) > 0 {
			td.Params = append([]contracts.TaskParamDTO(nil), meta.Params...)
		}
	}
}

// WithTaskConcurrency sets the concurrency limit for the task.
func WithTaskConcurrency(concurrency int) TaskOption {
	return func(td *TaskDefinition) {
		td.Concurrency = concurrency
	}
}

// WithTaskRetry sets the maximum retry count for the task.
func WithTaskRetry(retry int) TaskOption {
	return func(td *TaskDefinition) {
		td.Retry = retry
	}
}

// WithTaskTimeout sets the execution timeout for the task.
func WithTaskTimeout(timeout time.Duration) TaskOption {
	return func(td *TaskDefinition) {
		td.Timeout = timeout
	}
}

// WithTaskMetadata adds a key-value pair to the task metadata.
func WithTaskMetadata(key string, val any) TaskOption {
	return func(td *TaskDefinition) {
		if td.Metadata == nil {
			td.Metadata = make(map[string]any)
		}
		td.Metadata[key] = val
	}
}

// ToDTO converts TaskDefinition to contracts.TaskMetaDTO.
func (td TaskDefinition) ToDTO() contracts.TaskMetaDTO {
	taskType := td.Type
	if taskType == "" {
		taskType = td.Pattern
	}
	name := td.Name
	if name == "" {
		name = td.DisplayName
	}
	if name == "" {
		name = td.Pattern
	}
	displayName := td.DisplayName
	if displayName == "" {
		displayName = name
	}
	queue := td.Queue
	if queue == "" {
		queue = "default"
	}
	retryable := td.Retryable
	if !retryable && td.Retry > 0 {
		retryable = true
	}
	return contracts.TaskMetaDTO{
		Type:         taskType,
		AsynqTask:    td.Pattern,
		Name:         name,
		DisplayName:  displayName,
		Description:  td.Description,
		Category:     td.Category,
		SupportsTime: td.SupportsTime,
		Params:       td.Params,
		MaxRetry:     td.Retry,
		Timeout:      td.Timeout,
		Queue:        queue,
		Retryable:    retryable,
	}
}

// TaskExtension defines the interface for registering and querying background task handlers.
type TaskExtension interface {
	Register(pattern string, handler any, opts ...TaskOption)
	Tasks() []TaskDefinition
	Get(pattern string) (TaskDefinition, bool)
	Unregister(pattern string) bool
}

// TaskRegistry collects and manages task registrations.
type TaskRegistry struct {
	mu     sync.RWMutex
	tasks  []TaskDefinition
	lookup map[string]TaskDefinition
}

// NewTaskRegistry creates a new task registry.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{
		lookup: make(map[string]TaskDefinition),
	}
}

// Register registers a task pattern and its handler with optional configuration.
// A nil handler panics. A non-empty Type that is already used by another pattern panics.
func (t *TaskRegistry) Register(pattern string, handler any, opts ...TaskOption) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if isNilTaskHandler(handler) {
		panic(fmt.Sprintf("extpoints: nil handler for task pattern %q", pattern))
	}

	td := TaskDefinition{
		Pattern:  pattern,
		Handler:  handler,
		Metadata: make(map[string]any),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&td)
		}
	}
	if td.Type == "" {
		td.Type = pattern
	}

	for _, item := range t.tasks {
		if item.Pattern == pattern {
			continue
		}
		if item.Type == td.Type {
			panic(fmt.Sprintf("extpoints: duplicate task type %q (patterns %q and %q)", td.Type, item.Pattern, pattern))
		}
	}

	if existing, exists := t.lookup[pattern]; exists {
		if existing.Type != "" && existing.Type != pattern {
			delete(t.lookup, existing.Type)
		}
		for i, item := range t.tasks {
			if item.Pattern == pattern {
				t.tasks[i] = td
				break
			}
		}
	} else {
		t.tasks = append(t.tasks, td)
	}

	t.lookup[pattern] = td
	if td.Type != pattern {
		t.lookup[td.Type] = td
	}
}

func isNilTaskHandler(handler any) bool {
	if handler == nil {
		return true
	}
	v := reflect.ValueOf(handler)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

// Unregister removes a registered task definition by its pattern.
func (t *TaskRegistry) Unregister(pattern string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	td, ok := t.lookup[pattern]
	if !ok {
		return false
	}
	delete(t.lookup, td.Pattern)
	if td.Type != "" && td.Type != td.Pattern {
		delete(t.lookup, td.Type)
	}
	filtered := t.tasks[:0]
	for _, item := range t.tasks {
		if item.Pattern != td.Pattern {
			filtered = append(filtered, item)
		}
	}
	t.tasks = filtered
	return true
}

// Tasks returns a copy of all registered TaskDefinitions.
func (t *TaskRegistry) Tasks() []TaskDefinition {
	t.mu.RLock()
	defer t.mu.RUnlock()
	res := make([]TaskDefinition, len(t.tasks))
	copy(res, t.tasks)
	return res
}

// Get retrieves a task definition by its pattern or admin type identifier.
func (t *TaskRegistry) Get(pattern string) (TaskDefinition, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	td, ok := t.lookup[pattern]
	return td, ok
}
