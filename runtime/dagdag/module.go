package dagdag

import (
	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/iancoleman/strcase"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// TODO: sourcemap.

// ToModule converts Earthfile into Dagger Module type.
func ToModule(ef *earthfile.Earthfile) *dagger.Module {
	module := dag.TypeDef().
		WithObject(ef.ModuleName).
		WithConstructor(
			dag.Function("New", dag.TypeDef().WithObject(ef.ModuleName)).
				WithArg(
					"dockerUnixSock", dag.TypeDef().WithObject("Socket").WithOptional(true),
				),
		)

	for _, target := range ef.Targets {
		module = module.WithFunction(ToFunction(target))
	}

	return dag.Module().WithObject(module)
}

func ToFunction(target *earthfile.Target) *dagger.Function {
	returnType := dag.TypeDef().WithKind(dagger.TypeDefKindVoidKind)
	_, hasOutput := target.Output()
	if hasOutput {
		returnType = dag.TypeDef().WithObject("Container")
	}

	fn := dag.Function(strcase.ToCamel(target.Name), returnType).WithDescription(target.Doc)

	for name, argopt := range target.Args {
		kind := dag.TypeDef().WithKind(dagger.TypeDefKindStringKind)
		opts := dagger.FunctionWithArgOpts{Description: argopt.Doc}

		if !argopt.Required {
			kind = kind.WithOptional(true)
		}

		if argopt.DefaultValue != "" {
			opts.DefaultValue = dagger.JSON(argopt.DefaultValue)
		}

		fn = fn.WithArg(strcase.ToLowerCamel(name), kind, opts)
	}

	return fn
}
