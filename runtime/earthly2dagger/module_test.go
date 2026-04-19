package earthly2dagger

import (
	"context"
	"encoding/json"
	"testing"

	"dagger.io/dagger"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// argsByName collects function args into a map keyed by arg name for
// order-independent lookup. The range target.Args map has non-deterministic
// iteration order, so registered arg order in the Dagger function is not
// guaranteed. Tests that check specific arg properties must look up by name.
func argsByName(ctx context.Context, t *testctx.T, fns []dagger.Function) map[string]*dagger.FunctionArg {
	t.Helper()
	result := map[string]*dagger.FunctionArg{}
	for i := range fns {
		fn := &fns[i]
		args, err := fn.Args(ctx)
		require.NoError(t, err)
		for j := range args {
			arg := &args[j]
			name, err := arg.Name(ctx)
			require.NoError(t, err)
			result[name] = arg
		}
	}
	return result
}

func TestModule(t *testing.T) {
	testctx.New(t,
		testctx.WithParallel(),
	).RunTests(&ModuleSuite{})
}

type ModuleSuite struct{}

func (suite *ModuleSuite) TestToModule(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/to-module", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)
	require.Len(t, objects, 1)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)
	require.Len(t, functions, 2)

	name, err := functions[0].Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "build", name)

	name, err = functions[1].Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "imageA", name)
}

func (suite *ModuleSuite) TestArgument(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	args, err := functions[0].Args(ctx)
	require.NoError(t, err)

	t.Run("arguments", func(ctx context.Context, t *testctx.T) {
		require.Len(t, args, 3)

		name, err := args[0].Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "name", name)

		name, err = args[1].Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "tag", name)

		name, err = args[2].Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "argDefault", name)

		for _, arg := range args {
			assertTypeDef4(ctx, t, arg.TypeDef(), dagger.TypeDefKindStringKind)
		}
	})

	t.Run("optional argument", func(ctx context.Context, t *testctx.T) {
		optional, err := args[0].TypeDef().Optional(ctx)
		require.NoError(t, err)
		require.True(t, optional)
	})

	t.Run("required argument", func(ctx context.Context, t *testctx.T) {
		optional, err := args[1].TypeDef().Optional(ctx)
		require.NoError(t, err)
		require.False(t, optional)
	})

	t.Run("default value", func(ctx context.Context, t *testctx.T) {
		jsonValue, err := args[2].DefaultValue(ctx)
		require.NoError(t, err)
		var value string
		require.NoError(t, json.Unmarshal([]byte(jsonValue), &value))
		require.Equal(t, "default value", value)
	})
}

// TestArgNameCasing verifies that ARG names declared in SCREAMING_SNAKE_CASE
// are registered in the Dagger module as lowerCamelCase, which is the
// convention expected by the toEarthlyArgs roundtrip in main.go.
func (suite *ModuleSuite) TestArgNameCasing(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	byName := argsByName(ctx, t, functions)

	// ARG NAME → "name" (single word, all lower)
	_, ok := byName["name"]
	require.True(t, ok, "expected Dagger arg 'name' from ARG NAME")

	// ARG TAG (required) → "tag"
	_, ok = byName["tag"]
	require.True(t, ok, "expected Dagger arg 'tag' from ARG --required TAG")

	// ARG ARG_DEFAULT → "argDefault" (lowerCamelCase of SCREAMING_SNAKE)
	_, ok = byName["argDefault"]
	require.True(t, ok, "expected Dagger arg 'argDefault' from ARG ARG_DEFAULT")
}

// TestArgOptionalityAndDefault verifies the optional/required semantics and
// default value JSON encoding for each ARG variant.
func (suite *ModuleSuite) TestArgOptionalityAndDefault(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	byName := argsByName(ctx, t, functions)

	t.Run("optional arg has no default", func(ctx context.Context, t *testctx.T) {
		arg, ok := byName["name"]
		require.True(t, ok)

		optional, err := arg.TypeDef().Optional(ctx)
		require.NoError(t, err)
		require.True(t, optional, "ARG NAME (no default, no --required) must be optional")

		dv, err := arg.DefaultValue(ctx)
		require.NoError(t, err)
		require.Empty(t, dv, "ARG NAME with no default should have no DefaultValue")
	})

	t.Run("required arg is not optional and has no default", func(ctx context.Context, t *testctx.T) {
		arg, ok := byName["tag"]
		require.True(t, ok)

		optional, err := arg.TypeDef().Optional(ctx)
		require.NoError(t, err)
		require.False(t, optional, "ARG --required TAG must not be optional")

		dv, err := arg.DefaultValue(ctx)
		require.NoError(t, err)
		require.Empty(t, dv, "required ARG should have no DefaultValue registered")
	})

	t.Run("default value is JSON-encoded string", func(ctx context.Context, t *testctx.T) {
		arg, ok := byName["argDefault"]
		require.True(t, ok)

		optional, err := arg.TypeDef().Optional(ctx)
		require.NoError(t, err)
		require.True(t, optional, "ARG with default must be optional")

		jsonValue, err := arg.DefaultValue(ctx)
		require.NoError(t, err)

		// DefaultValue must be a valid JSON string, not raw text.
		var decoded string
		require.NoError(t, json.Unmarshal([]byte(jsonValue), &decoded),
			"DefaultValue must be a JSON-encoded string, got: %s", jsonValue)
		require.Equal(t, "default value", decoded)
	})
}

// TestArgDocumentation verifies that doc comments on ARGs are propagated
// into the Dagger function argument description.
func (suite *ModuleSuite) TestArgDocumentation(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	byName := argsByName(ctx, t, functions)

	arg, ok := byName["imageName"]
	require.True(t, ok, "expected Dagger arg 'imageName' from documented-args target")

	desc, err := arg.Description(ctx)
	require.NoError(t, err)
	require.Equal(t, "The image name.", desc)
}

// TestArgAllStringKind verifies that all ARGs are registered as StringKind
// regardless of their naming or default value.
func (suite *ModuleSuite) TestArgAllStringKind(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	byName := argsByName(ctx, t, functions)

	for _, arg := range byName {
		assertTypeDef4(ctx, t, arg.TypeDef(), dagger.TypeDefKindStringKind)
	}
}

func (suite *ModuleSuite) TestReturnVoidType(ctx context.Context, t *testctx.T) {
	t.Run("no SAVE IMAGE", func(ctx context.Context, t *testctx.T) {
		module := moduleFromPath(ctx, t, "../earthfile/testdata/simple", "simple")

		objects, err := module.Objects(ctx)
		require.NoError(t, err)

		functions, err := objects[0].AsObject().Functions(ctx)
		require.NoError(t, err)

		assertTypeDef4(ctx, t, functions[0].ReturnType(), dagger.TypeDefKindVoidKind)
	})
}

func (suite *ModuleSuite) TestReturContainerType(ctx context.Context, t *testctx.T) {
	t.Run("has SAVE IMAGE", func(ctx context.Context, t *testctx.T) {
		module := moduleFromPath(ctx, t, "testdata/container-type", "simple")

		objects, err := module.Objects(ctx)
		require.NoError(t, err)

		functions, err := objects[0].AsObject().Functions(ctx)
		require.NoError(t, err)

		assertTypeDef5(ctx, t, functions[0].ReturnType(), dagger.TypeDefKindObjectKind, "Container")
	})

	t.Run("SAVE IMAGE inside IF statement", func(ctx context.Context, t *testctx.T) {
		module := moduleFromPath(ctx, t, "testdata/save-image-if-stmt", "simple")

		objects, err := module.Objects(ctx)
		require.NoError(t, err)

		functions, err := objects[0].AsObject().Functions(ctx)
		require.NoError(t, err)

		assertTypeDef5(ctx, t, functions[0].ReturnType(), dagger.TypeDefKindObjectKind, "Container")
		assertTypeDef5(ctx, t, functions[1].ReturnType(), dagger.TypeDefKindObjectKind, "Container")
	})
}

// TestGlobalArgInFunction verifies that global ARGs from the base recipe are
// exposed as optional parameters on every Dagger function in the module.
func (suite *ModuleSuite) TestGlobalArgInFunction(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/global-args", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)
	require.Len(t, functions, 1)

	byName := argsByName(ctx, t, functions)

	// REGISTRY is a global ARG with default "docker.io" → must be optional.
	registry, ok := byName["registry"]
	require.True(t, ok, "expected Dagger arg 'registry' from ARG --global REGISTRY")

	optional, err := registry.TypeDef().Optional(ctx)
	require.NoError(t, err)
	require.True(t, optional, "global ARG REGISTRY must be optional")

	jsonValue, err := registry.DefaultValue(ctx)
	require.NoError(t, err)
	var decoded string
	require.NoError(t, json.Unmarshal([]byte(jsonValue), &decoded))
	require.Equal(t, "docker.io", decoded)

	// VERSION is a global ARG with no default → optional but no DefaultValue.
	version, ok := byName["version"]
	require.True(t, ok, "expected Dagger arg 'version' from ARG --global VERSION")

	optional, err = version.TypeDef().Optional(ctx)
	require.NoError(t, err)
	require.True(t, optional, "global ARG VERSION must be optional")

	dv, err := version.DefaultValue(ctx)
	require.NoError(t, err)
	require.Empty(t, dv, "global ARG VERSION with no default should have no DefaultValue")
}

// TestBuiltinArgSkipped verifies that Earthly built-in ARGs declared in a
// target do not appear as Dagger function parameters.
func (suite *ModuleSuite) TestBuiltinArgSkipped(ctx context.Context, t *testctx.T) {
	module := moduleFromPath(ctx, t, "testdata/arguments", "simple")

	objects, err := module.Objects(ctx)
	require.NoError(t, err)

	functions, err := objects[0].AsObject().Functions(ctx)
	require.NoError(t, err)

	byName := argsByName(ctx, t, functions)

	// Built-in ARGs must be absent from the Dagger function parameters.
	_, hasTargetPlatform := byName["targetplatform"]
	require.False(t, hasTargetPlatform, "TARGETPLATFORM is a builtin and must not be a Dagger param")
	_, hasEarthlyCi := byName["earthlyCi"]
	require.False(t, hasEarthlyCi, "EARTHLY_CI is a builtin and must not be a Dagger param")

	// The regular user ARG must still be present.
	_, hasUserArg := byName["userArg"]
	require.True(t, hasUserArg, "USER_ARG is a regular ARG and must be a Dagger param")
}

func assertTypeDef4(ctx context.Context, t *testctx.T, typeDef *dagger.TypeDef, kind dagger.TypeDefKind) {
	t.Helper()

	k, err := typeDef.Kind(ctx)
	require.NoError(t, err)
	require.Equal(t, kind, k)
}

func assertTypeDef5(ctx context.Context, t *testctx.T, typeDef *dagger.TypeDef, kind dagger.TypeDefKind, name string) {
	t.Helper()

	assertTypeDef4(ctx, t, typeDef, kind)

	if kind == dagger.TypeDefKindObjectKind {
		n, err := typeDef.AsObject().Name(ctx)
		require.NoError(t, err)
		require.Equal(t, name, n)
	}
}

func moduleFromPath(ctx context.Context, t *testctx.T, path string, modname string) *dagger.Module {
	t.Helper()

	earthfile, err := earthfile.New(ctx, path, modname)
	require.NoError(t, err)

	module, err := ToModule(earthfile).Sync(ctx)
	require.NoError(t, err)

	return module

}
