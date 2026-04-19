package earthly

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/earthly/earthly/ast/spec"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
)

// Interpreter walks an Earthfile Target AST and emits native Dagger API calls.
//
// One Interpreter instance is scoped to a single Earthfile build context
// plus the parsed Earthfile so it can resolve cross-target references like
// `+other-target/artifact`.
type Interpreter struct {
	// Source is the build context used for local COPY operations.
	Source *dagger.Directory

	// Earthfile is the parsed Earthfile used to resolve `+target` references.
	Earthfile *earthfile.Earthfile

	// cache memoizes already-built targets within a single top-level call so
	// that `COPY +foo/a` and `COPY +foo/b` only build `+foo` once.
	cache map[string]*dagger.Container
}

// NewInterpreter creates an Interpreter for the given build context and parsed Earthfile.
func NewInterpreter(src *dagger.Directory, ef *earthfile.Earthfile) *Interpreter {
	return &Interpreter{
		Source:    src,
		Earthfile: ef,
		cache:     map[string]*dagger.Container{},
	}
}

// Build walks target.Ast.Recipe and returns the final container state.
// Callers who want an image output return this container as-is.
// Callers who want an artifact should call Directory/File on the result.
func (i *Interpreter) Build(ctx context.Context, target *earthfile.Target, args map[string]string) (*dagger.Container, error) {
	if c, ok := i.cache[target.Name]; ok {
		return c, nil
	}

	// Merge resolved args in precedence order (lowest to highest):
	//   1. Global ARG defaults from the base recipe.
	//   2. Target-specific ARG defaults.
	//   3. Caller-provided overrides.
	resolved := map[string]string{}
	for name, opt := range i.Earthfile.GlobalArgs {
		if opt.DefaultValue != "" {
			resolved[name] = opt.DefaultValue
		}
	}
	for name, opt := range target.Args {
		if opt.DefaultValue != "" {
			resolved[name] = opt.DefaultValue
		}
	}
	for k, v := range args {
		resolved[k] = v
	}

	var ctr *dagger.Container
	var err error

	for _, stmt := range target.Ast.Recipe {
		ctr, err = i.evalStatement(ctx, ctr, stmt, resolved)
		if err != nil {
			return nil, err
		}
	}

	if ctr == nil {
		return nil, fmt.Errorf("target %q produced no container (missing FROM?)", target.Name)
	}

	i.cache[target.Name] = ctr
	return ctr, nil
}

func (i *Interpreter) evalStatement(ctx context.Context, ctr *dagger.Container, stmt spec.Statement, args map[string]string) (*dagger.Container, error) {
	switch {
	case stmt.Command != nil:
		return i.evalCommand(ctx, ctr, stmt.Command, args)
	case stmt.If != nil:
		return i.evalIf(ctx, ctr, stmt.If, args)
	case stmt.With != nil:
		// WITH DOCKER requires a Docker daemon — not supported in native mode.
		return nil, fmt.Errorf("WITH DOCKER is not supported in native Dagger translation")
	default:
		return ctr, nil
	}
}

// evalBlock walks a slice of statements sequentially, threading container state.
func (i *Interpreter) evalBlock(ctx context.Context, ctr *dagger.Container, block spec.Block, args map[string]string) (*dagger.Container, error) {
	var err error
	for _, stmt := range block {
		ctr, err = i.evalStatement(ctx, ctr, stmt, args)
		if err != nil {
			return nil, err
		}
	}
	return ctr, nil
}

// evalIf implements IF ... ELSE IF ... ELSE ... END.
//
// The condition expression is run as a shell command inside the current
// container; an exit code of 0 means the condition is true. The matching
// branch body is then evaluated and its resulting container state is returned.
func (i *Interpreter) evalIf(ctx context.Context, ctr *dagger.Container, stmt *spec.IfStatement, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("IF before FROM")
	}

	taken, err := i.evalCondition(ctx, ctr, stmt.Expression, stmt.ExecMode, args)
	if err != nil {
		return nil, fmt.Errorf("IF condition: %w", err)
	}
	if taken {
		return i.evalBlock(ctx, ctr, stmt.IfBody, args)
	}

	for _, elseIf := range stmt.ElseIf {
		taken, err = i.evalCondition(ctx, ctr, elseIf.Expression, elseIf.ExecMode, args)
		if err != nil {
			return nil, fmt.Errorf("ELSE IF condition: %w", err)
		}
		if taken {
			return i.evalBlock(ctx, ctr, elseIf.Body, args)
		}
	}

	if stmt.ElseBody != nil {
		return i.evalBlock(ctx, ctr, *stmt.ElseBody, args)
	}

	return ctr, nil
}

// evalCondition runs the condition expression in the container and returns
// true if the exit code is 0. The expression slice is joined and executed via
// sh -c (shell mode) or used directly as argv (exec mode).
func (i *Interpreter) evalCondition(ctx context.Context, ctr *dagger.Container, expression []string, execMode bool, args map[string]string) (bool, error) {
	raw := stripFlags(expression)
	if len(raw) == 0 {
		return false, fmt.Errorf("empty IF expression")
	}

	c := withArgsAsEnv(ctr, args)

	var execArgs []string
	if execMode {
		for _, a := range raw {
			execArgs = append(execArgs, expandArgs(a, args))
		}
	} else {
		script := expandArgs(strings.Join(raw, " "), args)
		execArgs = []string{"sh", "-c", script}
	}

	exitCode, err := c.WithExec(execArgs, dagger.ContainerWithExecOpts{
		Expect: dagger.ReturnTypeAny,
	}).ExitCode(ctx)
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

func (i *Interpreter) evalCommand(ctx context.Context, ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	switch cmd.Name {
	case "FROM":
		return i.cmdFrom(ctx, cmd, args)
	case "RUN":
		return i.cmdRun(ctr, cmd, args)
	case "COPY":
		return i.cmdCopy(ctx, ctr, cmd, args)
	case "ENV":
		return i.cmdEnv(ctr, cmd, args)
	case "WORKDIR":
		return i.cmdWorkdir(ctr, cmd, args)
	case "USER":
		return i.cmdUser(ctr, cmd, args)
	case "LABEL":
		return i.cmdLabel(ctr, cmd, args)
	case "ENTRYPOINT":
		if ctr == nil {
			return nil, fmt.Errorf("ENTRYPOINT before FROM")
		}
		return ctr.WithEntrypoint(cmd.Args), nil
	case "CMD":
		if ctr == nil {
			return nil, fmt.Errorf("CMD before FROM")
		}
		return ctr.WithDefaultArgs(cmd.Args), nil
	case "ARG":
		// ARGs are pre-resolved into `args` by the caller.
		return ctr, nil
	case "SAVE IMAGE":
		// Signal only — the caller returns the current container as the image.
		return ctr, nil
	case "SAVE ARTIFACT":
		// No-op during build walk. Artifact extraction happens via the
		// returned container's Directory/File methods.
		return ctr, nil
	case "EXPOSE":
		return i.cmdExpose(ctr, cmd)
	case "VOLUME", "SHELL", "STOPSIGNAL", "HEALTHCHECK":
		// Unsupported metadata commands — silently skip.
		return ctr, nil
	default:
		return nil, fmt.Errorf("unsupported Earthly command %q", cmd.Name)
	}
}

// cmdFrom handles the FROM instruction.
func (i *Interpreter) cmdFrom(ctx context.Context, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("FROM requires an image argument")
	}
	image := expandArgs(cmd.Args[0], args)

	// Cross-target FROM: `FROM +base`
	if strings.HasPrefix(image, "+") {
		targetName := strings.TrimPrefix(image, "+")
		refTarget := i.lookupTarget(targetName)
		if refTarget == nil {
			return nil, fmt.Errorf("FROM: unknown local target %q", image)
		}
		return i.Build(ctx, refTarget, map[string]string{})
	}

	return dag.Container().From(image), nil
}

// cmdRun handles the RUN instruction.
func (i *Interpreter) cmdRun(ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("RUN before FROM")
	}

	// Inject resolved ARGs as env vars so `$FOO` in shell commands resolves.
	c := withArgsAsEnv(ctr, args)

	// Strip unsupported RUN flags (--push, --no-cache, --secret, etc.).
	raw := stripFlags(cmd.Args)
	if len(raw) == 0 {
		return ctr, nil
	}

	var execArgs []string
	if cmd.ExecMode {
		// Exec form: args are already the argv.
		for _, a := range raw {
			execArgs = append(execArgs, expandArgs(a, args))
		}
	} else {
		// Shell form: join and wrap in sh -c.
		script := expandArgs(strings.Join(raw, " "), args)
		execArgs = []string{"sh", "-c", script}
	}

	return c.WithExec(execArgs), nil
}

// cmdCopy handles the COPY instruction.
func (i *Interpreter) cmdCopy(ctx context.Context, ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("COPY before FROM")
	}

	// Strip flags (--dir, --chown, --keep-ts, etc.) and work with positional args.
	positional := stripFlags(cmd.Args)
	if len(positional) < 2 {
		return nil, fmt.Errorf("COPY: need at least src and dest")
	}

	dest := expandArgs(positional[len(positional)-1], args)
	srcs := positional[:len(positional)-1]

	for _, src := range srcs {
		src = expandArgs(src, args)

		if strings.HasPrefix(src, "+") {
			// Cross-target artifact: `+target/path`
			var err error
			ctr, err = i.copyFromTarget(ctx, ctr, src, dest)
			if err != nil {
				return nil, err
			}
			continue
		}

		// Local path from build context.
		if looksLikeDir(src) {
			ctr = ctr.WithDirectory(dest, i.Source.Directory(src))
		} else {
			ctr = ctr.WithFile(dest, i.Source.File(src))
		}
	}
	return ctr, nil
}

// copyFromTarget resolves a `+target/artifact-path` reference.
func (i *Interpreter) copyFromTarget(ctx context.Context, ctr *dagger.Container, ref, dest string) (*dagger.Container, error) {
	// ref is like `+build/out/bin` — split targetName from artifact path.
	withoutPlus := strings.TrimPrefix(ref, "+")
	parts := strings.SplitN(withoutPlus, "/", 2)
	targetName := parts[0]
	artifactPath := "/"
	if len(parts) == 2 {
		artifactPath = "/" + parts[1]
	}

	refTarget := i.lookupTarget(targetName)
	if refTarget == nil {
		return nil, fmt.Errorf("COPY: unknown target %q", targetName)
	}

	built, err := i.Build(ctx, refTarget, map[string]string{})
	if err != nil {
		return nil, err
	}

	if looksLikeDir(artifactPath) {
		return ctr.WithDirectory(dest, built.Directory(artifactPath)), nil
	}
	return ctr.WithFile(dest, built.File(artifactPath)), nil
}

// cmdEnv handles the ENV instruction.
func (i *Interpreter) cmdEnv(ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("ENV before FROM")
	}
	k, v, err := parseKV(cmd.Args)
	if err != nil {
		return nil, fmt.Errorf("ENV: %w", err)
	}
	return ctr.WithEnvVariable(k, expandArgs(v, args)), nil
}

// cmdWorkdir handles the WORKDIR instruction.
func (i *Interpreter) cmdWorkdir(ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("WORKDIR before FROM")
	}
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("WORKDIR requires a path")
	}
	return ctr.WithWorkdir(expandArgs(cmd.Args[0], args)), nil
}

// cmdUser handles the USER instruction.
func (i *Interpreter) cmdUser(ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("USER before FROM")
	}
	if len(cmd.Args) == 0 {
		return nil, fmt.Errorf("USER requires a username")
	}
	return ctr.WithUser(expandArgs(cmd.Args[0], args)), nil
}

// cmdLabel handles the LABEL instruction.
func (i *Interpreter) cmdLabel(ctr *dagger.Container, cmd *spec.Command, args map[string]string) (*dagger.Container, error) {
	if ctr == nil {
		return ctr, nil
	}
	for _, arg := range cmd.Args {
		k, v, ok := strings.Cut(arg, "=")
		if !ok {
			continue
		}
		ctr = ctr.WithLabel(k, expandArgs(v, args))
	}
	return ctr, nil
}

// cmdExpose handles the EXPOSE instruction.
func (i *Interpreter) cmdExpose(ctr *dagger.Container, cmd *spec.Command) (*dagger.Container, error) {
	if ctr == nil {
		return nil, fmt.Errorf("EXPOSE before FROM")
	}
	for _, arg := range cmd.Args {
		portStr, proto, hasProto := strings.Cut(arg, "/")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("EXPOSE: invalid port %q: %w", portStr, err)
		}
		opts := dagger.ContainerWithExposedPortOpts{}
		if hasProto {
			switch strings.ToLower(proto) {
			case "tcp":
				opts.Protocol = dagger.NetworkProtocolTcp
			case "udp":
				opts.Protocol = dagger.NetworkProtocolUdp
			default:
				return nil, fmt.Errorf("EXPOSE: unsupported protocol %q", proto)
			}
		}
		ctr = ctr.WithExposedPort(port, opts)
	}
	return ctr, nil
}

// lookupTarget finds a target by its Earthfile name (not camelCase).
func (i *Interpreter) lookupTarget(name string) *earthfile.Target {
	// TargetsMap is keyed by camelCase; scan by Earthfile name too.
	for _, t := range i.Earthfile.Targets {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// withArgsAsEnv injects all resolved args as env vars so shell commands
// can reference them via `$VAR_NAME`.
func withArgsAsEnv(ctr *dagger.Container, args map[string]string) *dagger.Container {
	for k, v := range args {
		ctr = ctr.WithEnvVariable(k, v)
	}
	return ctr
}

// expandArgs substitutes `$NAME` and `${NAME}` references using the resolved
// args map. Unknown variables expand to empty string (shell semantics).
func expandArgs(s string, args map[string]string) string {
	return os.Expand(s, func(k string) string {
		return args[k]
	})
}

// parseKV parses `KEY=VALUE` or `KEY VALUE` forms used by ENV.
func parseKV(xs []string) (string, string, error) {
	if len(xs) == 0 {
		return "", "", fmt.Errorf("missing key")
	}
	if len(xs) == 1 {
		k, v, ok := strings.Cut(xs[0], "=")
		if !ok {
			return "", "", fmt.Errorf("expected KEY=VALUE, got %q", xs[0])
		}
		return k, v, nil
	}
	// `ENV KEY VALUE` form
	return xs[0], strings.Join(xs[1:], " "), nil
}

// looksLikeDir returns true when the path looks like a directory reference.
func looksLikeDir(p string) bool {
	if strings.HasSuffix(p, "/") {
		return true
	}
	base := p
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		base = p[idx+1:]
	}
	// No dot extension → treat as directory.
	return !strings.Contains(base, ".")
}

// stripFlags removes leading `--flag` tokens from a args slice, returning
// only positional arguments. This is a best-effort pass-1 approach; a full
// implementation would parse each command's specific flag set.
func stripFlags(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, a := range xs {
		if strings.HasPrefix(a, "--") {
			continue
		}
		out = append(out, a)
	}
	return out
}
