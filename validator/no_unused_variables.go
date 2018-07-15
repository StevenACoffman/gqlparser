package validator

import (
	"github.com/vektah/gqlparser/ast"
)

func init() {
	addRule("NoUnusedVariables", func(observers *Events, addError addErrFunc) {

		var variableNameUsed map[string]bool

		observers.OnOperation(func(walker *Walker, operation *ast.OperationDefinition) {
			variableNameUsed = make(map[string]bool)
		})

		observers.OnOperationLeave(func(walker *Walker, operation *ast.OperationDefinition) {
			for _, varDef := range operation.VariableDefinitions {
				if variableNameUsed[string(varDef.Variable)] {
					continue
				}

				if operation.Name != "" {
					addError(Message(`Variable "$%s" is never used in operation "%s".`, varDef.Variable, operation.Name))
				} else {
					addError(Message(`Variable "$%s" is never used.`, varDef.Variable))
				}
			}

			variableNameUsed = nil
		})

		observers.OnValue(func(walker *Walker, valueType ast.Type, def *ast.Definition, value ast.Value) {
			if variableNameUsed == nil {
				// not in operation context
				return
			}
			variable, isVariable := value.(ast.Variable)
			if !isVariable {
				return
			}
			variableNameUsed[string(variable)] = true
		})
	})
}
