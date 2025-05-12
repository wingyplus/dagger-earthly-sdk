package earthfile

import (
	"context"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/earthly/earthly/ast"
	"github.com/earthly/earthly/ast/spec"
	"github.com/iancoleman/strcase"
)

type Earthfile struct {
	Ast        spec.Earthfile
	ModuleName string
}

// Initiate Earthfile from path.
func New(ctx context.Context, path string, modname string) (*Earthfile, error) {
	ast, err := ast.Parse(ctx, path, true)
	if err != nil {
		return nil, err
	}
	return &Earthfile{
		Ast:        ast,
		ModuleName: modname,
	}, nil
}

// ToModule translate Earthly Earthfile into Dagger Module.
func (ef *Earthfile) ToModule() *dagger.Module {
	// TODO: sourcemap.
	module := dag.TypeDef().WithObject(ef.ModuleName)

	for _, target := range ef.Ast.Targets {
		fn := dag.Function(strcase.ToCamel(target.Name), dag.TypeDef().WithObject("Container"))
		for _, statement := range target.Recipe {
			cmd := statement.Command
			if cmd.Name == "ARG" {
				name := cmd.Args[0]
				required := false
				if cmd.Args[0] == "--required" {
					name = cmd.Args[1]
					required = true
				}

				kind := dag.TypeDef().WithKind(dagger.TypeDefKindStringKind)
				if !required {
					kind = kind.WithOptional(true)
				}

				fn = fn.WithArg(
					strcase.ToLowerCamel(name),
					kind,
					dagger.FunctionWithArgOpts{Description: cmd.Docs},
				)
			}
		}

		module = module.WithFunction(fn)
	}

	return dag.Module().WithObject(module)
}
