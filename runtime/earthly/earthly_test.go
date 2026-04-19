package earthly

import (
	"context"
	"io"
	"strings"
	"testing"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

func TestEarthly(t *testing.T) {
	testctx.New(t, testctx.WithParallel()).RunTests(&EarthlySuite{})
}

type EarthlySuite struct{}

// -- Basic container operations -------------------------------------------

func (s *EarthlySuite) TestSimpleRun(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo 'Hello, world!'
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)
	// Targets without SAVE IMAGE now return the final container state.
	_, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return even without SAVE IMAGE")
}

func (s *EarthlySuite) TestFromOnly(ctx context.Context, t *testctx.T) {
	// C1: A target with only a FROM instruction (no RUN, no SAVE IMAGE) must
	// return a usable *dagger.Container pointing at the pulled image.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return for FROM-only target")

	// Verify the container is functional — alpine ships with /etc/os-release.
	out, err := ctr.WithExec([]string{"sh", "-c", "cat /etc/os-release"}).Stdout(ctx)
	require.NoError(t, err)
	require.Contains(t, out, "Alpine", "FROM-only container should be a valid alpine image")
}

func (s *EarthlySuite) TestNoSaveImageWorkdir(ctx context.Context, t *testctx.T) {
	// A2: A target that ends with WORKDIR (no SAVE IMAGE) must return a
	// container with the working directory correctly set.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    WORKDIR /myapp
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	dir, err := ctr.Workdir(ctx)
	require.NoError(t, err)
	require.Equal(t, "/myapp", dir)
}

func (s *EarthlySuite) TestNoSaveImageEnv(ctx context.Context, t *testctx.T) {
	// A3: A target that ends with ENV (no SAVE IMAGE) must return a container
	// where the environment variable is visible to subsequent commands.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ENV SERVICE=myservice
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.WithExec([]string{"sh", "-c", "echo $SERVICE"}).Stdout(ctx)
	require.NoError(t, err)
	require.Equal(t, "myservice\n", out)
}

func (s *EarthlySuite) TestNoSaveImageUser(ctx context.Context, t *testctx.T) {
	// A4: A target that ends with USER (no SAVE IMAGE) must return a container
	// with the active user set to the specified user.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    USER nobody
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	user, err := ctr.User(ctx)
	require.NoError(t, err)
	require.Equal(t, "nobody", user)
}

func (s *EarthlySuite) TestWorkdir(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    WORKDIR /app
    SAVE IMAGE workdir-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)
	dir, err := ctr.Workdir(ctx)
	require.NoError(t, err)
	require.Equal(t, "/app", dir)
}

func (s *EarthlySuite) TestEnv(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ENV GREETING=hello
    SAVE IMAGE env-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)
	out, err := ctr.WithExec([]string{"sh", "-c", "echo $GREETING"}).Stdout(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello\n", out)
}

func (s *EarthlySuite) TestLabel(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    LABEL org.opencontainers.image.version=1.0.0
    SAVE IMAGE label-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)
	labels, err := ctr.Labels(ctx)
	require.NoError(t, err)

	var found bool
	for _, l := range labels {
		key, err := l.Name(ctx)
		require.NoError(t, err)
		if key == "org.opencontainers.image.version" {
			val, err := l.Value(ctx)
			require.NoError(t, err)
			require.Equal(t, "1.0.0", val)
			found = true
		}
	}
	require.True(t, found, "label org.opencontainers.image.version not found")
}

func (s *EarthlySuite) TestEntrypointAndCmd(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ENTRYPOINT ["sh", "-c"]
    CMD ["echo hi"]
    SAVE IMAGE entrypoint-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)
	ep, err := ctr.Entrypoint(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"sh", "-c"}, ep)

	def, err := ctr.DefaultArgs(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"echo hi"}, def)
}

// -- ARG handling ---------------------------------------------------------

func (s *EarthlySuite) TestArgDefault(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG GREETING=world
    FROM alpine
    RUN echo "Hello, $GREETING" > /out.txt
    SAVE IMAGE arg-default-test
`)
	// Do not pass GREETING — default should be used.
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "Hello, world\n", out)
}

func (s *EarthlySuite) TestArgRequired(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG --required NAME
    FROM alpine
    RUN echo "Hello, $NAME" > /out.txt
    SAVE IMAGE arg-required-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"NAME": "John Wick"})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "Hello, John Wick\n", out)
}

func (s *EarthlySuite) TestArgOverridesDefault(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG GREETING=world
    FROM alpine
    RUN echo "Hello, $GREETING" > /out.txt
    SAVE IMAGE arg-override-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"GREETING": "Dagger"})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "Hello, Dagger\n", out)
}

func (s *EarthlySuite) TestArgInFromImage(ctx context.Context, t *testctx.T) {
	// ARG used as the image reference in FROM $BASE_IMAGE.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG BASE_IMAGE=alpine
    FROM $BASE_IMAGE
    RUN echo "base=$BASE_IMAGE" > /base.txt
    SAVE IMAGE arg-from-test
`)
	// Use the default.
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/base.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "base=alpine\n", out)
}

func (s *EarthlySuite) TestArgInWorkdir(ctx context.Context, t *testctx.T) {
	// ARG substitution inside WORKDIR.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG APP_DIR=/workspace
    FROM alpine
    WORKDIR $APP_DIR
    SAVE IMAGE arg-workdir-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	dir, err := ret.(*dagger.Container).Workdir(ctx)
	require.NoError(t, err)
	require.Equal(t, "/workspace", dir)
}

func (s *EarthlySuite) TestArgInWorkdirOverride(ctx context.Context, t *testctx.T) {
	// Caller overrides the WORKDIR ARG.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG APP_DIR=/workspace
    FROM alpine
    WORKDIR $APP_DIR
    SAVE IMAGE arg-workdir-override-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"APP_DIR": "/app"})
	require.NoError(t, err)

	dir, err := ret.(*dagger.Container).Workdir(ctx)
	require.NoError(t, err)
	require.Equal(t, "/app", dir)
}

func (s *EarthlySuite) TestArgInEnvValue(ctx context.Context, t *testctx.T) {
	// ARG value substituted into ENV right-hand side.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG VERSION=1.0.0
    FROM alpine
    ENV APP_VERSION=$VERSION
    SAVE IMAGE arg-env-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).WithExec([]string{"sh", "-c", "echo $APP_VERSION"}).Stdout(ctx)
	require.NoError(t, err)
	require.Equal(t, "1.0.0\n", out)
}

func (s *EarthlySuite) TestArgBraceSyntaxInRun(ctx context.Context, t *testctx.T) {
	// Verify ${VAR} brace expansion works in RUN commands.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG PREFIX=hello
    FROM alpine
    RUN echo "${PREFIX}_world" > /out.txt
    SAVE IMAGE arg-brace-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello_world\n", out)
}

func (s *EarthlySuite) TestMultiArgPartialOverride(ctx context.Context, t *testctx.T) {
	// Target with 3 ARGs: 1 required (must be supplied), 2 with defaults.
	// Caller supplies only the required arg — the defaults should still apply.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG --required NAME
    ARG GREETING=Hello
    ARG PUNCT=!
    FROM alpine
    RUN echo "$GREETING, $NAME$PUNCT" > /out.txt
    SAVE IMAGE multi-arg-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"NAME": "World"})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "Hello, World!\n", out)
}

func (s *EarthlySuite) TestArgOverrideWithDifferentValue(ctx context.Context, t *testctx.T) {
	// Caller overrides one of multiple args while others keep defaults.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    ARG GREETING=Hello
    ARG NAME=World
    FROM alpine
    RUN echo "$GREETING, $NAME!" > /out.txt
    SAVE IMAGE multi-override-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"GREETING": "Hi"})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "Hi, World!\n", out)
}

func (s *EarthlySuite) TestBaseTargetArgDefaultAppliesOnFromTarget(ctx context.Context, t *testctx.T) {
	// FROM +baseimage where baseimage has ARG with default — the default must apply
	// since copyFromTarget/cmdFrom passes empty args to the base build.
	src, ef := sourceFromString(t, `VERSION 0.8

baseimage:
    ARG LABEL=base-label
    FROM alpine
    RUN echo "$LABEL" > /label.txt

app:
    FROM +baseimage
    SAVE IMAGE from-target-arg-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("App"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/label.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "base-label\n", out)
}

func (s *EarthlySuite) TestGlobalArgDefault(ctx context.Context, t *testctx.T) {
	// Global ARG default must be available in every target without being
	// explicitly passed by the caller.
	src, ef := sourceFromString(t, `VERSION 0.8

ARG --global REGISTRY=docker.io

build:
    FROM alpine
    RUN echo "$REGISTRY" > /registry.txt
    SAVE IMAGE global-arg-default-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/registry.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "docker.io\n", out)
}

func (s *EarthlySuite) TestGlobalArgOverride(ctx context.Context, t *testctx.T) {
	// Caller-provided value takes precedence over the global ARG default.
	src, ef := sourceFromString(t, `VERSION 0.8

ARG --global REGISTRY=docker.io

build:
    FROM alpine
    RUN echo "$REGISTRY" > /registry.txt
    SAVE IMAGE global-arg-override-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{"REGISTRY": "ghcr.io"})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/registry.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "ghcr.io\n", out)
}

// -- COPY from build context ----------------------------------------------

func (s *EarthlySuite) TestCopyFile(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    COPY hello.txt /hello.txt
    SAVE IMAGE copy-file-test
`)
	// Add a file to the build context.
	src = src.WithNewFile("hello.txt", "hello from context\n")

	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/hello.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello from context\n", out)
}

func (s *EarthlySuite) TestCopyDirectory(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    COPY src/ /app/
    SAVE IMAGE copy-dir-test
`)
	src = src.WithNewFile("src/main.go", "package main\n")

	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/app/main.go").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "package main\n", out)
}

// -- SAVE IMAGE -----------------------------------------------------------

func (s *EarthlySuite) TestSaveImage(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo 'hello' > /a.txt
    SAVE IMAGE dagger-earthly-sdk/image
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	contents, err := ctr.File("/a.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "hello\n", contents)
}

func (s *EarthlySuite) TestSaveImagePush(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo 'push variant' > /b.txt
    SAVE IMAGE --push example.com/myimage:latest
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return for SAVE IMAGE --push")

	contents, err := ctr.File("/b.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "push variant\n", contents)
}

// -- Cross-target references ----------------------------------------------

func (s *EarthlySuite) TestFromTarget(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

baseimage:
    FROM alpine
    RUN echo 'base layer' > /base.txt

app:
    FROM +baseimage
    RUN echo 'app layer' > /app.txt
    SAVE IMAGE from-target-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("App"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)

	base, err := ctr.File("/base.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "base layer\n", base)

	app, err := ctr.File("/app.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "app layer\n", app)
}

func (s *EarthlySuite) TestCopyFromTarget(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

builder:
    FROM alpine
    RUN mkdir -p /out && echo 'artifact content' > /out/result.txt
    SAVE ARTIFACT /out/result.txt

app:
    FROM alpine
    COPY +builder/out/result.txt /result.txt
    SAVE IMAGE copy-from-target-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("App"), Args{})
	require.NoError(t, err)

	contents, err := ret.(*dagger.Container).File("/result.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "artifact content\n", contents)
}

// -- EXPOSE ---------------------------------------------------------------

func (s *EarthlySuite) TestExposePort(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    EXPOSE 80 443/tcp 53/udp
    SAVE IMAGE expose-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr := ret.(*dagger.Container)
	ports, err := ctr.ExposedPorts(ctx)
	require.NoError(t, err)

	type portProto struct {
		port  int
		proto dagger.NetworkProtocol
	}
	var got []portProto
	for _, p := range ports {
		num, err := p.Port(ctx)
		require.NoError(t, err)
		proto, err := p.Protocol(ctx)
		require.NoError(t, err)
		got = append(got, portProto{num, proto})
	}

	require.ElementsMatch(t, []portProto{
		{80, dagger.NetworkProtocolTcp},
		{443, dagger.NetworkProtocolTcp},
		{53, dagger.NetworkProtocolUdp},
	}, got)
}

// -- IF control flow ------------------------------------------------------

func (s *EarthlySuite) TestIfTrueBranch(ctx context.Context, t *testctx.T) {
	// Condition is true (exit 0) — the if-body should run.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    IF [ "true" = "true" ]
        RUN echo "true-branch" > /branch.txt
    ELSE
        RUN echo "false-branch" > /branch.txt
    END
    SAVE IMAGE if-true-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/branch.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "true-branch\n", out)
}

func (s *EarthlySuite) TestIfFalseBranch(ctx context.Context, t *testctx.T) {
	// Condition is false (exit 1) — the else-body should run.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    IF [ "true" = "false" ]
        RUN echo "true-branch" > /branch.txt
    ELSE
        RUN echo "false-branch" > /branch.txt
    END
    SAVE IMAGE if-false-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/branch.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "false-branch\n", out)
}

func (s *EarthlySuite) TestIfElseIfBranch(ctx context.Context, t *testctx.T) {
	// Primary condition is false; else-if condition is true.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ARG VALUE=b
    IF [ "$VALUE" = "a" ]
        RUN echo "branch-a" > /branch.txt
    ELSE IF [ "$VALUE" = "b" ]
        RUN echo "branch-b" > /branch.txt
    ELSE
        RUN echo "branch-other" > /branch.txt
    END
    SAVE IMAGE if-elseif-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/branch.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "branch-b\n", out)
}

func (s *EarthlySuite) TestIfNoMatchFallsToElse(ctx context.Context, t *testctx.T) {
	// Neither if nor else-if matches; else body runs.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ARG VALUE=z
    IF [ "$VALUE" = "a" ]
        RUN echo "branch-a" > /branch.txt
    ELSE IF [ "$VALUE" = "b" ]
        RUN echo "branch-b" > /branch.txt
    ELSE
        RUN echo "branch-other" > /branch.txt
    END
    SAVE IMAGE if-else-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/branch.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "branch-other\n", out)
}

func (s *EarthlySuite) TestIfNoElseBodyOnFalse(ctx context.Context, t *testctx.T) {
	// Condition is false and there is no ELSE — container state unchanged.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo "before" > /out.txt
    IF [ "a" = "b" ]
        RUN echo "inside-if" > /out.txt
    END
    SAVE IMAGE if-no-else-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "before\n", out)
}

func (s *EarthlySuite) TestIfNestedBlocks(ctx context.Context, t *testctx.T) {
	// Nested IF inside IF body.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    ARG OUTER=yes
    ARG INNER=yes
    IF [ "$OUTER" = "yes" ]
        IF [ "$INNER" = "yes" ]
            RUN echo "both" > /out.txt
        ELSE
            RUN echo "outer-only" > /out.txt
        END
    ELSE
        RUN echo "neither" > /out.txt
    END
    SAVE IMAGE if-nested-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "both\n", out)
}

func (s *EarthlySuite) TestNoSaveImageIfTakenBranch(ctx context.Context, t *testctx.T) {
	// A5: Target with IF control flow (no SAVE IMAGE) — the container from the
	// taken branch must be returned.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    IF [ "a" = "a" ]
        RUN echo "if-branch" > /out.txt
    ELSE
        RUN echo "else-branch" > /out.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "if-branch\n", out)
}

func (s *EarthlySuite) TestNoSaveImageIfElseBranch(ctx context.Context, t *testctx.T) {
	// A6: Target with IF/ELSE control flow (no SAVE IMAGE) — when the condition
	// is false the ELSE branch container must be returned.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    IF [ "a" = "b" ]
        RUN echo "if-branch" > /out.txt
    ELSE
        RUN echo "else-branch" > /out.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "else-branch\n", out)
}

func (s *EarthlySuite) TestNoSaveImageIfFalseNoElse(ctx context.Context, t *testctx.T) {
	// A7: Target with IF, false condition, no ELSE (no SAVE IMAGE) — the
	// pre-IF container state must be returned unchanged.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo "before" > /out.txt
    IF [ "a" = "b" ]
        RUN echo "unreachable" > /out.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "before\n", out)
}

// -- FOR control flow -----------------------------------------------------

func (s *EarthlySuite) TestForSingleItem(ctx context.Context, t *testctx.T) {
	// Loop over a single item — body executes once.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    FOR item IN one
        RUN echo "$item" >> /items.txt
    END
    SAVE IMAGE for-single-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/items.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "one\n", out)
}

func (s *EarthlySuite) TestForMultipleItems(ctx context.Context, t *testctx.T) {
	// Loop over multiple literal items.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    FOR item IN alpha beta gamma
        RUN echo "$item" >> /items.txt
    END
    SAVE IMAGE for-multi-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/items.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "alpha\nbeta\ngamma\n", out)
}

func (s *EarthlySuite) TestForEmptyList(ctx context.Context, t *testctx.T) {
	// Empty list expression — body never executes; container unchanged.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo "base" > /out.txt
    FOR item IN $(printf "")
        RUN echo "$item" >> /out.txt
    END
    SAVE IMAGE for-empty-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "base\n", out)
}

func (s *EarthlySuite) TestNoSaveImageForLoop(ctx context.Context, t *testctx.T) {
	// A8: Target with a FOR loop (no SAVE IMAGE) — the post-loop container
	// state, accumulating all iterations, must be returned.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    FOR item IN x y z
        RUN echo "$item" >> /items.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/items.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "x\ny\nz\n", out)
}

// -- TRY / CATCH / FINALLY control flow ----------------------------------

func (s *EarthlySuite) TestTrySuccessPath(ctx context.Context, t *testctx.T) {
	// Try body succeeds — catch should NOT run, finally SHOULD run.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    TRY
        RUN echo "try-ok" > /result.txt
    FINALLY
        RUN echo "finally" >> /result.txt
    END
    SAVE IMAGE try-success-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/result.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "try-ok\nfinally\n", out)
}

func (s *EarthlySuite) TestTryFailureCatchRuns(ctx context.Context, t *testctx.T) {
	// Try body fails — catch SHOULD run and recover; finally SHOULD run.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN echo "init" > /result.txt
    TRY
        RUN exit 1
    CATCH
        RUN echo "caught" >> /result.txt
    FINALLY
        RUN echo "finally" >> /result.txt
    END
    SAVE IMAGE try-catch-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/result.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "init\ncaught\nfinally\n", out)
}

func (s *EarthlySuite) TestNoSaveImageTryCatch(ctx context.Context, t *testctx.T) {
	// A9: Target with TRY/CATCH (no SAVE IMAGE) — when the try body fails the
	// catch body runs and the resulting container must be returned.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    TRY
        RUN exit 1
    CATCH
        RUN echo "caught" > /out.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/out.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "caught\n", out)
}

func (s *EarthlySuite) TestTryFinallyAlwaysRuns(ctx context.Context, t *testctx.T) {
	// Try fails, no catch, finally must still run (and final error propagates).
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    TRY
        RUN exit 1
    FINALLY
        RUN echo "cleanup" > /cleanup.txt
    END
    SAVE IMAGE try-finally-test
`)
	// The overall build should fail because try failed with no catch.
	_, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.Error(t, err)
}

// -- WAIT control flow ----------------------------------------------------

func (s *EarthlySuite) TestWaitForcesSync(ctx context.Context, t *testctx.T) {
	// Operations inside WAIT ... END are evaluated eagerly before continuing.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    WAIT
        RUN echo "inside-wait" > /wait.txt
    END
    RUN echo "after-wait" >> /wait.txt
    SAVE IMAGE wait-sync-test
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	out, err := ret.(*dagger.Container).File("/wait.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "inside-wait\nafter-wait\n", out)
}

func (s *EarthlySuite) TestNoSaveImageWait(ctx context.Context, t *testctx.T) {
	// A10: Target with WAIT (no SAVE IMAGE) — the post-WAIT container state
	// must be returned. WAIT forces synchronous evaluation before continuing.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    WAIT
        RUN echo "inside" > /w.txt
    END
`)
	ret, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.NoError(t, err)

	ctr, ok := ret.(*dagger.Container)
	require.True(t, ok, "expected *dagger.Container return")

	out, err := ctr.File("/w.txt").Contents(ctx)
	require.NoError(t, err)
	require.Equal(t, "inside\n", out)
}

func (s *EarthlySuite) TestWaitErrorPropagates(ctx context.Context, t *testctx.T) {
	// A failing command inside WAIT must propagate an error.
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    WAIT
        RUN exit 42
    END
`)
	_, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.Error(t, err)
}

// -- Error handling -------------------------------------------------------

func (s *EarthlySuite) TestRunError(ctx context.Context, t *testctx.T) {
	src, ef := sourceFromString(t, `VERSION 0.8

build:
    FROM alpine
    RUN exit 2
`)
	_, err := New().Invoke(ctx, src, ef, ef.TargetFromFunctionName("Build"), Args{})
	require.Error(t, err)
}

// -- Helpers --------------------------------------------------------------

func sourceFromString(t *testctx.T, earthfileContent string) (*dagger.Directory, *earthfile.Earthfile) {
	t.Helper()
	ctx := t.Context()
	src := dag.Directory().WithNewFile("/Earthfile", earthfileContent)
	ef, err := earthfile.NewFromOpts(
		ctx, "/",
		earthfile.FromReader(namedReaderCompat(strings.NewReader(earthfileContent))),
		"modname",
	)
	require.NoError(t, err)
	return src, ef
}

type namedReaderShim struct {
	io.ReadSeeker
}

func (n *namedReaderShim) Close() error { return nil }
func (n *namedReaderShim) Name() string { return "Earthfile" }

func namedReaderCompat(r io.ReadSeeker) earthfile.NamedReader {
	return &namedReaderShim{r}
}
