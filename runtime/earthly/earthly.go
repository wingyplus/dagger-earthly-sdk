package earthly

import (
	"context"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

const (
	workspacePath         = "/workspace"
	earthlyImage          = "earthly/earthly:v0.8.15@sha256:23131ae7a7fc57b7121614bad290a4e5b06233d5f862c2ea821385e4943bdb0b"
	earthlyBuildkitdImage = "earthly/buildkitd:v0.8.15@sha256:72f64a9098628483e89758ebff583ef5c0e7f9df88be7288d471a92648f9ec08"
)

type Args map[string]string

func New(dockerUnixSock *dagger.Socket) *Earthly {
	return &Earthly{DockerUnixSock: dockerUnixSock}
}

type Earthly struct {
	DockerUnixSock *dagger.Socket
}

// Invoke calls Earthly target.
//
// The method will returns a container once the target call `SAVE IMAGE`.
func (m *Earthly) Invoke(ctx context.Context, source *dagger.Directory, target *earthfile.Target, args Args) (*dagger.Container, error) {
	// TODO: convert oci tar to Dagger Container.
	cmd := []string{"earthly", "--ci", "--allow-privileged", "+" + target.Name}
	for k, v := range args {
		cmd = append(cmd, "--"+k, v)
	}

	_, err := m.Runtime(source).
		WithWorkdir(workspacePath).
		WithMountedDirectory(".", source).
		WithExec(cmd).
		Sync(ctx)

	// TODO: fixme
	return nil, err
}

func (m *Earthly) Runtime(source *dagger.Directory) *dagger.Container {
	config := `
global:
  tls_enabled: false
`
	ctr := dag.Container().From(earthlyImage).
		WithNewFile("/root/.earthly/config.yml", config).
		WithoutEntrypoint()

	if m.DockerUnixSock != nil {
		ctr = ctr.
			WithEnvVariable("DOCKER_HOST", "unix:///var/run/docker.sock")
	} else {
		ctr = ctr.
			WithServiceBinding("dockerd", m.DockerEngine()).
			WithEnvVariable("DOCKER_HOST", "tcp://dockerd:2375")
	}
	return ctr
	// WithServiceBinding("buildkitd", m.Buildkitd()).
	// 	WithEnvVariable("NO_BUILDKIT", "1").
	// 	WithEnvVariable("EARTHLY_BUILDKIT_HOST", "tcp://buildkitd:8372").
	// WithoutEntrypoint()
}

func (m *Earthly) DockerEngine() *dagger.Service {
	return dag.Container().
		From("docker:28-dind").
		WithExposedPort(2375).
		WithEntrypoint([]string{
			"dockerd",
			"--host=tcp://0.0.0.0:2375",
			"--host=unix:///var/run/docker.sock",
			"--tls=false",
		}).
		WithDefaultArgs([]string{}).
		AsService(dagger.ContainerAsServiceOpts{
			UseEntrypoint:                 true,
			ExperimentalPrivilegedNesting: true,
			InsecureRootCapabilities:      true,
		})
}

// Start the Earthly Buildkitd as a service.
func (m *Earthly) Buildkitd() *dagger.Service {
	return dag.Container().
		From(earthlyBuildkitdImage).
		WithEnvVariable("BUILDKIT_TCP_TRANSPORT_ENABLED", "true").
		WithEnvVariable("BUILDKIT_TLS_ENABLED", "false").
		WithExposedPort(8372).
		AsService(dagger.ContainerAsServiceOpts{
			UseEntrypoint:                 true,
			ExperimentalPrivilegedNesting: true,
			InsecureRootCapabilities:      true,
		})
}
