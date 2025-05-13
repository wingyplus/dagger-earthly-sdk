package earthfile

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEarthfile(t *testing.T) {
	earthfile, err := New(t.Context(), "testdata/simple", "simple")
	require.NoError(t, err)
	require.Equal(t, "simple", earthfile.ModuleName)

	require.Equal(t, "build", earthfile.Targets["Build"].Name)
	require.Equal(t, "", earthfile.Targets["Build"].Doc)
	require.Equal(t, map[string]ArgOpt{}, earthfile.Targets["Build"].Args)
}
