package earthfile

import "github.com/earthly/earthly/ast/spec"

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

func (t *Target) Output() (string, bool) {
	for _, statement := range t.Ast.Recipe {
		if cmd := statement.Command; cmd != nil {
			if cmd.Name == "SAVE IMAGE" {
				return cmd.Args[0], true
			}
		}
	}
	return "", false
}

func parseTargets(asts []spec.Target) (targets []*Target) {
	for _, ast := range asts {
		targets = append(targets, parseTarget(ast))
	}
	return
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
