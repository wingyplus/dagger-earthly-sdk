package earthly

import (
	"io"
	"strings"
	"testing"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/stretchr/testify/require"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// TODO: convert to testctx

func TestEarthly(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		source, earthfile := source(t, `VERSION 0.8

build:
		FROM alpine
		RUN echo 'Hello, world!'
`)

		earthly := New(nil)
		ret, err := earthly.Invoke(t.Context(), source, earthfile.TargetFromFunctionName("Build"), Args{})
		require.Nil(t, ret)
		require.NoError(t, err)
	})

	t.Run("pass arguments", func(t *testing.T) {
		source, earthfile := source(t, `VERSION 0.8

build:
		ARG --required NAME
		FROM alpine
		RUN echo "Hello, ${NAME}"
`)

		earthly := New(nil)
		_, err := earthly.Invoke(t.Context(), source, earthfile.TargetFromFunctionName("Build"), Args{"NAME": "John Wick"})
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		source, earthfile := source(t, `VERSION 0.8

build:
		FROM alpine
		RUN exit 2
`)

		earthly := New(nil)
		_, err := earthly.Invoke(t.Context(), source, earthfile.TargetFromFunctionName("Build"), Args{})
		require.Error(t, err)
	})

	t.Run("export container", func(t *testing.T) {
		source, earthfile := source(t, `VERSION 0.8

build:
		FROM alpine 
		RUN echo 'hello' > /a.txt 
		SAVE IMAGE dagger-earthly-sdk/image
`)

		earthly := New(nil)
		ret, err := earthly.Invoke(t.Context(), source, earthfile.TargetFromFunctionName("Build"), Args{})
		require.NoError(t, err)

		ctr, ok := ret.(*dagger.Container)
		require.True(t, ok)

		// Ensure file `/a.txt` exists in the container.
		contents, err := ctr.File("/a.txt").Contents(t.Context())
		require.NoError(t, err)
		require.Equal(t, "hello\n", contents)
	})
}

func source(t *testing.T, earthfileContent string) (*dagger.Directory, *earthfile.Earthfile) {
	t.Helper()
	ctx := t.Context()
	source := dag.Directory().WithNewFile("/Earthfile", earthfileContent)
	earthfile, err := earthfile.NewFromOpts(ctx, "/Earthfile", earthfile.FromReader(namedReaderCompat(strings.NewReader(earthfileContent))), "modname")
	require.NoError(t, err)
	return source, earthfile
}

type namedReaderShim struct {
	io.ReadSeeker
}

func (n *namedReaderShim) Close() error { return nil }

func (n *namedReaderShim) Name() string { return "Earthfile" }

// namedReaderCompat fill the NamedReader implementations to the io.ReadSeeker.
func namedReaderCompat(r io.ReadSeeker) earthfile.NamedReader {
	return &namedReaderShim{r}
}
