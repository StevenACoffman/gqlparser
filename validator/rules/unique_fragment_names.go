package validator

import (
	"github.com/dgraph-io/gqlparser/ast"
	. "github.com/dgraph-io/gqlparser/validator"
)

func init() {
	AddRule("UniqueFragmentNames", func(observers *Events, addError AddErrFunc) {
		seenFragments := map[string]bool{}

		observers.OnFragment(func(walker *Walker, fragment *ast.FragmentDefinition) {
			if seenFragments[fragment.Name] {
				addError(
					Message(`There can be only one fragment named "%s".`, fragment.Name),
					At(fragment.Position),
				)
			}
			seenFragments[fragment.Name] = true
		})
	})
}
