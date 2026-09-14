package scope

import (
	"fmt"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

type ScopeError struct {
	Cause   error
	Message string
}

// Error returns the error message
func (e *ScopeError) Error() string {
	return e.Message
}

// how the error is been caused by the underlying error
func (e *ScopeError) Unwrap() error {
	return e.Cause
}

// BuildScopeTree builds a scope tree from extracted entities.
// It mirrors the TS buildScopeTree: any panic thrown while building is
// recovered and returned as a *ScopeError wrapping the underlying cause.
func BuildScopeTree(entities []types.ExtractedEntity) (tree types.ScopeTree, err error) {
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*ScopeError)
			if !ok {
				cause, isErr := r.(error)
				if !isErr {
					cause = fmt.Errorf("%v", r)
				}
				se = &ScopeError{
					Message: fmt.Sprintf("Failed to build scope tree: %s", cause.Error()),
					Cause:   cause,
				}
			}
			err = se
		}
	}()

	return buildScopeTreeOrPanic(entities), nil
}

// buildScopeTreeOrPanic is the "inner try": any failure it hits is converted
// into a *ScopeError panic (mirroring `throw new ScopeError(...)`), which the
// outer recover in BuildScopeTree catches. Panics originating deeper in the
// build are wrapped here, then rethrown so the outer recover still sees them.
// This makes the try/catch win-win for both thrown panics and passed-through
// plain errors.
func buildScopeTreeOrPanic(entities []types.ExtractedEntity) types.ScopeTree {
	defer func() {
		if r := recover(); r != nil {
			cause, ok := r.(error)
			if !ok {
				cause = fmt.Errorf("%v", r)
			}
			panic(&ScopeError{
				Message: fmt.Sprintf("Failed to build scope tree: %s", cause.Error()),
				Cause:   cause,
			})
		}
	}()

	return BuildScopeTreeFromEntities(entities)
}
