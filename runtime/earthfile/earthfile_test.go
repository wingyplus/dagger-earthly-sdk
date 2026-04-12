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
