package earthfile

import (
	"github.com/earthly/earthly/ast/spec"
	"github.com/iancoleman/strcase"
)

type ArgOpt struct {
	Required bool
	Doc      string
}

type Target struct {
	Name string
	Doc  string
	Args map[string]ArgOpt
	Ast  spec.Target
	// TODO: sourcemap
}

// Output returns an image output specified in `SAVE IMAGE` instruction.
func (t *Target) Output() (string, bool) {
	for _, statement := range t.Ast.Recipe {
		if cmd := statement.Command; cmd != nil {
			if out, found := saveImageOutput(cmd); found {
				return out, found
			}
		}

		if stmt := statement.If; stmt != nil {
			for _, statement := range stmt.IfBody {
				if out, found := saveImageOutput(statement.Command); found {
					return out, found
				}
			}
			for _, statement := range *stmt.ElseBody {
				if out, found := saveImageOutput(statement.Command); found {
					return out, found
				}
			}
		}
	}
	return "", false
}

func saveImageOutput(cmd *spec.Command) (string, bool) {
	if cmd.Name == "SAVE IMAGE" {
		if cmd.Args[0] == "--push" {
			return cmd.Args[1], true
		}
		return cmd.Args[0], true
	}
	return "", false
}

func parseTarget(ast spec.Target) *Target {
	target := &Target{
		Name: ast.Name,
		Doc:  ast.Docs,
		Args: map[string]ArgOpt{},
		Ast:  ast,
	}
	for _, statement := range ast.Recipe {
		if cmd := statement.Command; cmd != nil {
			if cmd.Name == "ARG" {
				name := cmd.Args[0]
				required := false
				if cmd.Args[0] == "--required" {
					name = cmd.Args[1]
					required = true
				}
				target.Args[name] = ArgOpt{Required: required, Doc: cmd.Docs}
			}
		}
	}
	return target
}

type TargetsMap map[string]*Target

func parseTargetsMap(asts []spec.Target) (targets TargetsMap) {
	targets = make(TargetsMap)
	for _, ast := range asts {
		target := parseTarget(ast)
		targets[strcase.ToCamel(target.Name)] = target
	}
	return
}
