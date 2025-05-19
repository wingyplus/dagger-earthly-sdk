package dagdag

import (
	"context"
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
	earthfile, err := earthfile.New(ctx, "testdata/to-module", "simple")
	require.NoError(t, err)

	module, err := ToModule(earthfile).Sync(ctx)
	require.NoError(t, err)

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

func (suite *ModuleSuite) TestReturnVoidType(ctx context.Context, t *testctx.T) {
	t.Run("no SAVE IMAGE", func(ctx context.Context, t *testctx.T) {
		earthfile, err := earthfile.New(ctx, "../earthfile/testdata/simple", "simple")
		require.NoError(t, err)

		module, err := ToModule(earthfile).Sync(ctx)
		require.NoError(t, err)

		objects, err := module.Objects(ctx)
		require.NoError(t, err)

		functions, err := objects[0].AsObject().Functions(ctx)
		require.NoError(t, err)

		assertTypeDef4(ctx, t, functions[0].ReturnType(), dagger.TypeDefKindVoidKind)
	})
}

func (suite *ModuleSuite) TestReturContainerType(ctx context.Context, t *testctx.T) {
	t.Run("has SAVE IMAGE", func(ctx context.Context, t *testctx.T) {
		earthfile, err := earthfile.New(ctx, "testdata/container-type", "simple")
		require.NoError(t, err)

		module, err := ToModule(earthfile).Sync(ctx)
		require.NoError(t, err)

		objects, err := module.Objects(ctx)
		require.NoError(t, err)

		functions, err := objects[0].AsObject().Functions(ctx)
		require.NoError(t, err)

		assertTypeDef5(ctx, t, functions[0].ReturnType(), dagger.TypeDefKindObjectKind, "Container")
	})

	t.Run("SAVE IMAGE inside IF statement", func(ctx context.Context, t *testctx.T) {
		earthfile, err := earthfile.New(ctx, "testdata/save-image-if-stmt", "simple")
		require.NoError(t, err)

		module, err := ToModule(earthfile).Sync(ctx)
		require.NoError(t, err)

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
