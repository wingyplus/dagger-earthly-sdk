package earthly2dagger

import (
	"encoding/json"
	"sort"

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

	names := make([]string, 0, len(ef.Targets))
	for name := range ef.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		module = module.WithFunction(ToFunction(ef.Targets[name], ef.GlobalArgs))
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
	// All targets return *dagger.Container — the final container state after
	// executing the recipe. Targets with SAVE IMAGE return it as an image
	// handle; targets without return the working container at end of execution.
	returnType := dag.TypeDef().WithObject("Container")

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

	for _, name := range target.ArgOrder {
		if earthfile.IsBuiltinArg(name) {
			continue
		}
		argopt := target.Args[name]
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
