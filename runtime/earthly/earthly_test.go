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
	require.Nil(t, ret)
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

base:
    FROM alpine
    RUN echo 'base layer' > /base.txt

app:
    FROM +base
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
    RUN echo 'artifact content' > /out/result.txt
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
