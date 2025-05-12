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
	Targets    TargetsMap
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
		Targets:    parseTargetsMap(ast.Targets),
	}, nil
}

// ToModule translate Earthly Earthfile into Dagger Module.
func (ef *Earthfile) ToModule() *dagger.Module {
	// TODO: sourcemap.
	module := dag.TypeDef().WithObject(ef.ModuleName)

	for _, target := range ef.Targets {
		returnTypeKind := dag.TypeDef().WithKind(dagger.TypeDefKindVoidKind)
		_, hasOutput := target.Output()
		if hasOutput {
			returnTypeKind = dag.TypeDef().WithObject("Container")
		}

		fn := dag.Function(strcase.ToCamel(target.Name), returnTypeKind)

		for name, argopt := range target.Args {
			kind := dag.TypeDef().WithKind(dagger.TypeDefKindStringKind)
			if !argopt.Required {
				kind = kind.WithOptional(true)
			}

			fn = fn.WithArg(
				strcase.ToLowerCamel(name),
				kind,
				dagger.FunctionWithArgOpts{Description: argopt.Doc},
			)

			module = module.WithFunction(fn)
		}
	}

	return dag.Module().WithObject(module)
}

func (ef *Earthfile) TargetFromFunctionName(name string) *Target {
	return ef.Targets[name]
}
