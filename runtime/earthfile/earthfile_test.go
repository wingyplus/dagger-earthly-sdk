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
