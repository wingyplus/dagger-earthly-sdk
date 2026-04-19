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

// Invoke builds a target natively via the Interpreter and returns the final
// *dagger.Container state. All targets return a container: those with SAVE
// IMAGE return it as an image handle; those without return the working
// container at the end of recipe execution.
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

	// Sync forces eager evaluation of the pipeline so that runtime errors
	// (e.g. a failing RUN) surface here rather than lazily in the caller.
	ctr, err = ctr.Sync(ctx)
	if err != nil {
		return nil, err
	}

	return ctr, nil
}

// Source returns a Dagger Directory for the given host path, used as the
// build context passed to Invoke.
func Source(path string) *dagger.Directory {
	return dag.Host().Directory(path)
}
