package earthfile

import (
	"context"
	"testing"

	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

func TestEarthfile(t *testing.T) {
	testctx.New(t, testctx.WithParallel()).RunTests(&EarthfileSuite{})
}

type EarthfileSuite struct{}

func (suite *EarthfileSuite) TestCompile(ctx context.Context, t *testctx.T) {
	earthfile, err := New(t.Context(), "testdata/simple", "simple")
	require.NoError(t, err)
	require.Equal(t, "simple", earthfile.ModuleName)

	require.Equal(t, "build", earthfile.Targets["Build"].Name)
	require.Equal(t, "", earthfile.Targets["Build"].Doc)
	require.Equal(t, map[string]ArgOpt{}, earthfile.Targets["Build"].Args)
}

func (suite *EarthfileSuite) TestParseArguments(ctx context.Context, t *testctx.T) {
	earthfile, err := New(t.Context(), "testdata/args", "args")
	require.NoError(t, err)

	require.Equal(t, map[string]ArgOpt{
		"A": {Name: "A", DefaultValue: "", Required: false, Doc: ""},
	}, earthfile.Targets["OptionalArg"].Args)
	require.Equal(t, map[string]ArgOpt{
		"B": {Name: "B", DefaultValue: "", Required: true, Doc: ""},
	}, earthfile.Targets["RequiredArg"].Args)
	require.Equal(t, map[string]ArgOpt{
		"C": {Name: "C", DefaultValue: "D", Required: false, Doc: ""},
	}, earthfile.Targets["OptionalDefaultArg"].Args)
	require.Equal(t, map[string]ArgOpt{
		"C": {Name: "C", DefaultValue: "D=E", Required: false, Doc: ""},
	}, earthfile.Targets["OptionalDefaultArg2"].Args)
}

func (suite *EarthfileSuite) TestParseArgDoc(ctx context.Context, t *testctx.T) {
	ef, err := New(t.Context(), "testdata/args", "args")
	require.NoError(t, err)

	target := ef.Targets["DocumentedArg"]
	require.NotNil(t, target)
	arg, ok := target.Args["NAME"]
	require.True(t, ok, "expected ARG NAME to be present")
	require.Equal(t, "The name to greet.", arg.Doc)
}

func (suite *EarthfileSuite) TestParseMultipleArgs(ctx context.Context, t *testctx.T) {
	ef, err := New(t.Context(), "testdata/args", "args")
	require.NoError(t, err)

	target := ef.Targets["MultiArg"]
	require.NotNil(t, target)
	require.Equal(t, map[string]ArgOpt{
		"FOO": {Name: "FOO", DefaultValue: "", Required: true, Doc: ""},
		"BAR": {Name: "BAR", DefaultValue: "default-bar", Required: false, Doc: ""},
		"BAZ": {Name: "BAZ", DefaultValue: "", Required: false, Doc: ""},
	}, target.Args)
}

func (suite *EarthfileSuite) TestParseGlobalArgs(ctx context.Context, t *testctx.T) {
	ef, err := New(t.Context(), "testdata/global-args", "global")
	require.NoError(t, err)

	require.Len(t, ef.GlobalArgs, 2)

	registry, ok := ef.GlobalArgs["REGISTRY"]
	require.True(t, ok, "expected REGISTRY in GlobalArgs")
	require.Equal(t, "REGISTRY", registry.Name)
	require.Equal(t, "docker.io", registry.DefaultValue)
	require.True(t, registry.Global)

	version, ok := ef.GlobalArgs["VERSION"]
	require.True(t, ok, "expected VERSION in GlobalArgs")
	require.Equal(t, "VERSION", version.Name)
	require.Equal(t, "", version.DefaultValue)
	require.True(t, version.Global)
}

func (suite *EarthfileSuite) TestGlobalArgNotInTargetArgs(ctx context.Context, t *testctx.T) {
	ef, err := New(t.Context(), "testdata/global-args", "global")
	require.NoError(t, err)

	target := ef.Targets["Build"]
	require.NotNil(t, target)
	// Global ARGs live in ef.GlobalArgs, not in any target's Args map.
	_, registryInTarget := target.Args["REGISTRY"]
	require.False(t, registryInTarget, "REGISTRY is a global ARG and must not appear in target.Args")
}

func (suite *EarthfileSuite) TestIsBuiltinArg(ctx context.Context, t *testctx.T) {
	builtins := []string{
		"EARTHLY_CI", "EARTHLY_BUILD_SHA", "EARTHLY_GIT_HASH",
		"TARGETPLATFORM", "TARGETOS", "TARGETARCH", "TARGETVARIANT",
		"NATIVEPLATFORM", "NATIVEOS", "NATIVEARCH", "NATIVEVARIANT",
		"USERPLATFORM", "USEROS", "USERARCH", "USERVARIANT",
		"EARTHLY_FUTURE_ARG", // EARTHLY_ prefix catch-all
	}
	for _, name := range builtins {
		require.True(t, IsBuiltinArg(name), "expected %q to be a builtin ARG", name)
	}

	nonBuiltins := []string{"REGISTRY", "VERSION", "NAME", "TAG", "USER_DEFINED"}
	for _, name := range nonBuiltins {
		require.False(t, IsBuiltinArg(name), "expected %q to NOT be a builtin ARG", name)
	}
}

func (suite *EarthfileSuite) TestBuiltinArgParsedInTargetArgs(ctx context.Context, t *testctx.T) {
	// Builtin ARGs that appear in a target recipe ARE stored in target.Args
	// (the parser does not filter them). Filtering for Dagger schema happens
	// in dagdag.ToFunction via IsBuiltinArg.
	ef, err := New(t.Context(), "testdata/args", "args")
	require.NoError(t, err)

	target := ef.Targets["BuiltinArg"]
	require.NotNil(t, target)
	_, hasTargetPlatform := target.Args["TARGETPLATFORM"]
	require.True(t, hasTargetPlatform, "TARGETPLATFORM should be parsed into target.Args")
	_, hasEarthlyCi := target.Args["EARTHLY_CI"]
	require.True(t, hasEarthlyCi, "EARTHLY_CI should be parsed into target.Args")
	_, hasRealArg := target.Args["REAL_ARG"]
	require.True(t, hasRealArg, "REAL_ARG should be parsed into target.Args")
}
