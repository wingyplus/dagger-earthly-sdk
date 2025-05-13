package earthly

import (
	"testing"

	"dagger.io/dagger/dag"
	"github.com/stretchr/testify/require"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

func TestEarthly(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		source := dag.Directory().
			WithNewFile("/Earthfile", `VERSION 0.8

build:
		FROM alpine
		RUN echo 'Hello, world!'
`)

		target := &earthfile.Target{
			Name: "build",
		}

		earthly := New(nil)
		_, err := earthly.Invoke(t.Context(), source, target, Args{})
		require.NoError(t, err)
	})

	t.Run("pass arguments", func(t *testing.T) {
		source := dag.Directory().
			WithNewFile("/Earthfile", `VERSION 0.8

build:
		ARG --required NAME
		FROM alpine
		RUN echo "Hello, ${NAME}"
`)

		target := &earthfile.Target{
			Name: "build",
			Args: map[string]earthfile.ArgOpt{
				"NAME": {
					Required: true,
				},
			},
		}

		earthly := New(nil)
		_, err := earthly.Invoke(t.Context(), source, target, Args{"NAME": "John Wick"})
		require.NoError(t, err)
	})

	t.Run("error", func(t *testing.T) {
		source := dag.Directory().
			WithNewFile("/Earthfile", `VERSION 0.8

build:
		FROM alpine
		RUN exit 2
`)

		target := &earthfile.Target{
			Name: "build",
		}

		earthly := New(nil)
		_, err := earthly.Invoke(t.Context(), source, target, Args{"NAME": "John Wick"})
		require.Error(t, err)
	})
}
