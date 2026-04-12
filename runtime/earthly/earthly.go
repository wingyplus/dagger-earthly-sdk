package earthly

import (
	"context"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// Args holds the resolved build arguments passed to an Earthly target.
type Args map[string]string

// Earthly executes Earthfile targets by translating their instructions into
// native Dagger operations. No Earthly CLI, Docker daemon, or Buildkitd
// service is required.
type Earthly struct{}

// New creates an Earthly executor.
func New() *Earthly { return &Earthly{} }

// Invoke builds a target natively via the Interpreter and returns either a
// *dagger.Container (if the target has SAVE IMAGE) or nil on success.
func (m *Earthly) Invoke(
	ctx context.Context,
	source *dagger.Directory,
	ef *earthfile.Earthfile,
	target *earthfile.Target,
	args Args,
) (any, error) {
	interp := NewInterpreter(source, ef)

	ctr, err := interp.Build(ctx, target, args)
	if err != nil {
		return nil, err
	}

	if _, hasImage := target.Output(); hasImage {
		return ctr, nil
	}

	// Void target — force evaluation so errors surface.
	_, err = ctr.Sync(ctx)
	return nil, err
}

// Source returns a Dagger Directory for the given host path, used as the
// build context passed to Invoke.
func Source(path string) *dagger.Directory {
	return dag.Host().Directory(path)
}
