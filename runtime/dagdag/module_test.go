package dagdag

import (
	"context"
	"encoding/json"
	"testing"

	"dagger.io/dagger"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// TODO: add arguments test.

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
