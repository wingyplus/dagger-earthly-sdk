package dagdag

import (
	"encoding/json"

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
			dag.Function("New", dag.TypeDef().WithObject(ef.ModuleName)),
		)

	for _, target := range ef.Targets {
		module = module.WithFunction(ToFunction(target, ef.GlobalArgs))
	}

	return dag.Module().WithObject(module)
}

// ToFunction converts an Earthfile Target into a Dagger Function declaration.
//
// Global ARGs (from the Earthfile base recipe) are added as optional parameters
// to every function so callers can override them per-invocation.
//
// Earthly built-in ARGs (EARTHLY_*, TARGETPLATFORM, etc.) are silently skipped
// because their values are injected by the runtime, not supplied by users.
func ToFunction(target *earthfile.Target, globals map[string]earthfile.ArgOpt) *dagger.Function {
	returnType := dag.TypeDef().WithKind(dagger.TypeDefKindVoidKind)
	_, hasOutput := target.Output()
	if hasOutput {
		returnType = dag.TypeDef().WithObject("Container")
	}

	fn := dag.Function(strcase.ToCamel(target.Name), returnType).WithDescription(target.Doc)

	// Global ARGs are always optional — callers may override the base-recipe
	// default, but are never required to.
	for name, argopt := range globals {
		if earthfile.IsBuiltinArg(name) {
			continue
		}
		kind := dag.TypeDef().WithKind(dagger.TypeDefKindStringKind).WithOptional(true)
		opts := dagger.FunctionWithArgOpts{Description: argopt.Doc}
		if argopt.DefaultValue != "" {
			jsonBytes, _ := json.Marshal(argopt.DefaultValue)
			opts.DefaultValue = dagger.JSON(jsonBytes)
		}
		fn = fn.WithArg(strcase.ToLowerCamel(name), kind, opts)
	}

	for name, argopt := range target.Args {
		if earthfile.IsBuiltinArg(name) {
			continue
		}
		kind := dag.TypeDef().WithKind(dagger.TypeDefKindStringKind)
		opts := dagger.FunctionWithArgOpts{Description: argopt.Doc}

		if !argopt.Required {
			kind = kind.WithOptional(true)
		}

		if argopt.DefaultValue != "" {
			jsonBytes, _ := json.Marshal(argopt.DefaultValue)
			opts.DefaultValue = dagger.JSON(jsonBytes)
		}

		fn = fn.WithArg(strcase.ToLowerCamel(name), kind, opts)
	}

	return fn
}
