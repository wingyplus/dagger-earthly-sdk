// Earthly runtime for Dagger. 🚀

package main

import (
	"context"
	"fmt"

	"github.com/wingyplus/dagger-earthly-sdk/internal/dagger"
)

func New(
	// +optional
	// +defaultPath="./runtime"
	sdkSourceDir *dagger.Directory,
) *EarthlySdk {
	return &EarthlySdk{
		RuntimeSourceDir: sdkSourceDir,
		Container:        dag.Container(),
	}
}

type EarthlySdk struct {
	RuntimeSourceDir *dagger.Directory

	// +private
	Container *dagger.Container
}

// ModuleRuntime implements runtime api.
func (m *EarthlySdk) ModuleRuntime(
	ctx context.Context,
	modSource *dagger.ModuleSource,
	introspectionJSON *dagger.File,
) (*dagger.Container, error) {
	subPath, err := modSource.SourceSubpath(ctx)
	if err != nil {
		return nil, err
	}

	ctr := m.Container.
		From("alpine").
		WithFile("/usr/local/bin/earthly-sdk-runtime", m.Runtime()).
		WithWorkdir("/src").
		WithMountedDirectory(".", modSource.ContextDirectory()).
		WithWorkdir(subPath).
		WithEntrypoint([]string{
			"earthly-sdk-runtime", fmt.Sprintf("/src/%s", subPath),
		})
	return ctr, nil
}

// Codegen implements runtime api.
func (m *EarthlySdk) Codegen(
	ctx context.Context,
	modSource *dagger.ModuleSource,
	introspectionJSON *dagger.File,
) (*dagger.GeneratedCode, error) {
	// TODO: do not override existing Earthfile.

	subPath, err := modSource.SourceSubpath(ctx)
	if err != nil {
		return nil, err
	}

	ctr := m.Container.
		From("alpine").
		WithWorkdir("/src").
		WithMountedDirectory(".", modSource.ContextDirectory()).
		WithFile(subPath+"/Earthfile", dag.CurrentModule().Source().File("templates/Earthfile"))
	return dag.GeneratedCode(ctr.Directory("/src")).
			WithVCSGeneratedPaths([]string{}).
			WithVCSIgnoredPaths([]string{}),
		nil
}

// Runtime create a runtime binary for running the a function.
func (m *EarthlySdk) Runtime() *dagger.File {
	return m.Container.
		From("golang:1.24-alpine").
		WithEnvVariable("CGO_ENABLED", "0").
		WithWorkdir("/runtime").
		WithDirectory(".", m.RuntimeSourceDir).
		WithExec([]string{"go", "build", "-o", "bin/runtime", "."}).
		File("bin/runtime")
}
