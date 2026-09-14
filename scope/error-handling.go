package scope

import (
	"fmt"

	"github.com/RITIK-KHARYA/go-code-chunk/types"
)

// ScopeError - error building scope tree
type ScopeError struct {
	Message string
	Cause   error
}

func (e *ScopeError) Error() string {
	return e.Message
}

func (e *ScopeError) Unwrap() error {
	return e.Cause
}

// BuildScopeTree builds a scope tree from extracted entities.
// Recovers any unexpected panic during build and returns it as *ScopeError,
// so a bug deep in tree-building never crashes the caller.
func BuildScopeTree(entities []types.ExtractedEntity) (tree types.ScopeTree, err error) {
	defer func() {
		if r := recover(); r != nil {
			cause, ok := r.(error)
			if !ok {
				cause = fmt.Errorf("%v", r)
			}
			err = &ScopeError{
				Message: fmt.Sprintf("failed to build scope tree: %s", cause.Error()),
				Cause:   cause,
			}
		}
	}()

	return BuildScopeTreeFromEntities(entities), nil
}

// final tree returning final for both
// if successfull then tree
// otherwise return (generate) empty tree

func BuildScopeTreeSync(entities []types.ExtractedEntity) (tree types.ScopeTree) {
	defer func() {
		if r := recover(); r != nil {
			tree = types.ScopeTree{
				Root:        []*types.ScopeNode{},
				Imports:     []types.ExtractedEntity{},
				Exports:     []types.ExtractedEntity{},
				AllEntities: entities,
			}
		}
	}()

	return BuildScopeTreeFromEntities(entities)
}
