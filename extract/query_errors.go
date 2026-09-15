package extract

import (
	"fmt"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// QueryLoadError is returned when a query cannot be compiled for a language.
type QueryLoadError struct {
	Language types.Language
	Message  string
	Cause    error
}

func (e *QueryLoadError) Error() string {
	return fmt.Sprintf("query load failed for %s: %s", e.Language, e.Message)
}

func (e *QueryLoadError) Unwrap() error {
	return e.Cause
}

// QueryExecutionError is returned when query execution fails at runtime.
type QueryExecutionError struct {
	Message string
	Cause   error
}

func (e *QueryExecutionError) Error() string {
	return "query execution failed: " + e.Message
}

func (e *QueryExecutionError) Unwrap() error {
	return e.Cause
}
