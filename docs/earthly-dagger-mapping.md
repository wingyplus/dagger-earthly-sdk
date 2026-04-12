# Earthly → Dagger API Mapping Reference

This document maps every Earthly keyword that can appear inside a target recipe to its
corresponding Dagger Go SDK call. It covers the full Earthly command surface, not just
what is currently implemented. See [progress.md](progress.md) for implementation status.

Sources: `earthly/ast/command/names.go`, `earthly/ast/commandflag/flags.go`,
`earthly/ast/spec/earthfile.go`, `earthly/docs/earthfile/earthfile.md`.

---

## Container origin

| Earthly                                            | Dagger Go SDK                                                                                                          | Notes                                                                 |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `FROM <image>`                                     | `dag.Container().From(image string)` → `*dagger.Container`                                                             | Starting point for most targets                                       |
| `FROM +<target>`                                   | Recursive `Interpreter.Build(ctx, target, args)` → `*dagger.Container`                                                 | Inherits another target's container                                   |
| `FROM DOCKERFILE [-f path] [--target stage] <ctx>` | `dag.Directory(ctx).DockerBuild(dagger.DirectoryDockerBuildOpts{Dockerfile, Target, BuildArgs})` → `*dagger.Container` | Interprets an existing Dockerfile                                     |
| `LOCALLY`                                          | No equivalent — Dagger modules are sandboxed                                                                           | Runs commands on the host; incompatible with Dagger's execution model |
| `GIT CLONE <url> <dest>`                           | `dag.Git(url).Branch(ref).Tree()` → `*dagger.Directory`; then `ctr.WithDirectory(dest, tree)`                          | `dagger.GitOpts` accepts auth token for private repos                 |

---

## Execution

| Earthly                                 | Dagger Go SDK                                                                      | Notes                                                                                            |
| --------------------------------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `RUN <cmd>` (shell form)                | `ctr.WithExec([]string{"sh", "-c", cmd})` → `*dagger.Container`                    | Default; uses `/bin/sh -c`                                                                       |
| `RUN ["a","b",...]` (exec form)         | `ctr.WithExec([]string{"a","b",...})` → `*dagger.Container`                        | Bypasses shell                                                                                   |
| `RUN --privileged`                      | `ctr.WithExec(args, dagger.ContainerWithExecOpts{InsecureRootCapabilities: true})` |                                                                                                  |
| `RUN --no-cache`                        | No direct equivalent                                                               | Dagger uses content-addressed caching; use `ctr.WithEnvVariable("CACHE_BUST", ts)` to invalidate |
| `RUN --push`                            | `ctr.Publish(ctx, ref)` after build completes                                      | Push-only side-effect; deferred to caller                                                        |
| `RUN --secret NAME=ref`                 | `ctr.WithSecretVariable(name, secret)` or `ctr.WithMountedSecret(path, secret)`    |                                                                                                  |
| `RUN --mount=type=cache,target=<path>`  | `ctr.WithMountedCache(path, dag.CacheVolume(key), opts)`                           |                                                                                                  |
| `RUN --mount=type=secret,target=<path>` | `ctr.WithMountedSecret(path, dag.SetSecret(name, val))`                            |                                                                                                  |
| `RUN --ssh`                             | `ctr.WithUnixSocket("/ssh-agent.sock", dag.Host().UnixSocket(agentPath))`          |                                                                                                  |
| `RUN --network=none`                    | No Dagger SDK API                                                                  | OCI network namespace control not exposed                                                        |
| `RUN --interactive`                     | No equivalent                                                                      | Interactive terminal not available in Dagger modules                                             |

---

## Filesystem / data

| Earthly                                      | Dagger Go SDK                                                                                          | Notes                                                                     |
| -------------------------------------------- | ------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------- |
| `COPY <file> <dest>` (local file)            | `ctr.WithFile(dest, sourceDir.File(src))` → `*dagger.Container`                                        |                                                                           |
| `COPY <dir>/ <dest>` (local dir)             | `ctr.WithDirectory(dest, sourceDir.Directory(src))` → `*dagger.Container`                              |                                                                           |
| `COPY --chown=user:group`                    | `dagger.ContainerWithFileOpts{Owner: "user:group"}` or `dagger.ContainerWithDirectoryOpts{Owner: ...}` |                                                                           |
| `COPY --chmod=mode`                          | `dagger.ContainerWithFileOpts{Permissions: mode}`                                                      |                                                                           |
| `COPY +target/<path> <dest>` (artifact dir)  | `builtCtr.Directory(artifactPath)` → `ctr.WithDirectory(dest, dir)`                                    |                                                                           |
| `COPY +target/<path> <dest>` (artifact file) | `builtCtr.File(artifactPath)` → `ctr.WithFile(dest, file)`                                             |                                                                           |
| `ADD <src> <dest>`                           | **Not supported by Earthly**                                                                           | Earthly docs mark this unsupported                                        |
| `SAVE ARTIFACT <path> [<name>]`              | `ctr.Directory(path)` → `*dagger.Directory` or `ctr.File(path)` → `*dagger.File`                       | Function return type: `dag.TypeDef().WithObject("Directory")` or `"File"` |
| `SAVE ARTIFACT ... AS LOCAL <hostPath>`      | `ctr.Directory(path).Export(ctx, hostPath)` or `ctr.File(path).Export(ctx, hostPath)`                  | Writes to caller's host                                                   |
| `SAVE IMAGE <tag>`                           | Return `*dagger.Container` from function                                                               | Function return type: `dag.TypeDef().WithObject("Container")`             |
| `SAVE IMAGE --push <tag>`                    | `ctr.Publish(ctx, tag)` → `(string, error)` returns the resolved digest                                |                                                                           |

---

## Variables

| Earthly                 | Dagger Go SDK (declaration)                                                                       | Dagger Go SDK (runtime)                                  | Notes                     |
| ----------------------- | ------------------------------------------------------------------------------------------------- | -------------------------------------------------------- | ------------------------- |
| `ARG <name>`            | `fn.WithArg(name, dag.TypeDef().WithKind(dagger.TypeDefKindStringKind).WithOptional(true), opts)` | `ctr.WithEnvVariable(name, val)` before `WithExec`       | Optional ARG              |
| `ARG --required <name>` | `fn.WithArg(name, dag.TypeDef().WithKind(dagger.TypeDefKindStringKind), opts)`                    | Same as above                                            | Required ARG              |
| `ARG <name>=<default>`  | `fn.WithArg(name, kind, dagger.FunctionWithArgOpts{DefaultValue: dagger.JSON(val)})`              | Same as above                                            | Default provided          |
| `ENV KEY=VALUE`         | N/A (not a function parameter)                                                                    | `ctr.WithEnvVariable(name, value)` → `*dagger.Container` | Baked into image          |
| `LET <name>=<value>`    | N/A                                                                                               | Go local variable in interpreter                         | Mutable local variable    |
| `SET <name>=<value>`    | N/A                                                                                               | Reassign Go local variable                               | Reassign a `LET` variable |

---

## Composition

| Earthly                                 | Dagger Go SDK                                                         | Notes                                    |
| --------------------------------------- | --------------------------------------------------------------------- | ---------------------------------------- |
| `BUILD +<target> [--build-arg KEY=VAL]` | `Interpreter.Build(ctx, target, args).Sync(ctx)`                      | Side-effect only; result discarded       |
| `DO +<Function> [KEY=VAL]`              | Inline-expand the `FUNCTION` block into the current interpreter state | No direct Dagger API; pure AST expansion |
| `IMPORT <ref> [AS <alias>]`             | Resolve alias when encountering `+alias/target` refs                  | No direct Dagger API                     |

---

## Flow control

These are AST statement forms (not leaf commands). They must be evaluated in Go with
the interpreter's container as the accumulator.

| Earthly                             | Dagger approach                                                                        | Implementation note          |
| ----------------------------------- | -------------------------------------------------------------------------------------- | ---------------------------- |
| `IF <expr> ... ELSE ... END`        | Run predicate via `ctr.WithExec(expr).ExitCode(ctx)`, then branch in Go                | `ExitCode == 0` → truthy     |
| `FOR <var> IN <expr> ... END`       | Evaluate list via `ctr.WithExec(expr).Stdout(ctx)`, split on separators, iterate in Go | Default separators: `\n\t `  |
| `WAIT ... END`                      | Force `ctr.Sync(ctx)` on all containers produced inside the block                      | Synchronisation barrier      |
| `TRY ... CATCH ... FINALLY ... END` | Wrap try-block in Go error handler; run catch/finally blocks accordingly               | Experimental Earthly feature |

---

## Caching

| Earthly                                   | Dagger Go SDK                                                                                                                | Notes                                       |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `CACHE [--sharing=...] [--id=...] <path>` | `ctr.WithMountedCache(path, dag.CacheVolume(key), dagger.ContainerWithMountedCacheOpts{Sharing: ...})` → `*dagger.Container` | `Sharing`: `Locked`, `Shared`, or `Private` |

---

## Image metadata

These Earthly commands map 1:1 onto Dockerfile image config.

| Earthly                 | Dagger Go SDK                                                                                                               | Notes                                                            |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| `WORKDIR <path>`        | `ctr.WithWorkdir(path)` → `*dagger.Container`                                                                               | Also affects subsequent RUN                                      |
| `USER <user>`           | `ctr.WithUser(user)` → `*dagger.Container`                                                                                  |                                                                  |
| `ENV KEY=VALUE`         | `ctr.WithEnvVariable(key, value)` → `*dagger.Container`                                                                     |                                                                  |
| `LABEL key=value`       | `ctr.WithLabel(name, value)` → `*dagger.Container`                                                                          |                                                                  |
| `ENTRYPOINT [...]`      | `ctr.WithEntrypoint(args []string)` → `*dagger.Container`                                                                   |                                                                  |
| `CMD [...]`             | `ctr.WithDefaultArgs(args []string)` → `*dagger.Container`                                                                  |                                                                  |
| `EXPOSE <port>[/proto]` | `ctr.WithExposedPort(port, dagger.ContainerWithExposedPortOpts{Protocol: dagger.NetworkProtocolTcp})` → `*dagger.Container` | Metadata-only in image context                                   |
| `VOLUME <path>`         | `ctr.WithMountedTemp(path)` (approximation)                                                                                 | No exact equivalent; OCI volume metadata not preserved in Dagger |
| `HEALTHCHECK CMD <cmd>` | No Dagger SDK API                                                                                                           | OCI healthcheck metadata not exposed                             |
| `HOST <hostname> <ip>`  | No Dagger SDK API                                                                                                           | `/etc/hosts` injection not exposed                               |
| `SHELL [...]`           | Interpreter state only                                                                                                      | Changes the shell wrapper used for subsequent RUN commands       |
| `STOPSIGNAL <signal>`   | No Dagger SDK API                                                                                                           | OCI stop signal metadata not exposed                             |

---

## Docker-in-Docker

| Earthly               | Dagger approach                                                                                                            | Notes                                                   |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------- |
| `WITH DOCKER ... END` | Spin a `docker:dind` service, bind it via `ctr.WithServiceBinding(alias, svc)` for the block's RUNs, then stop after `END` | `svc = dag.Container().From("docker:dind").AsService()` |

---

## Module schema API (declaration side)

These Dagger API calls are used in `runtime/dagdag/module.go` to declare the Dagger
module schema from parsed Earthfile targets — not executed at runtime.

| Purpose              | Dagger Go SDK call                                                                  |
| -------------------- | ----------------------------------------------------------------------------------- |
| Module object        | `dag.TypeDef().WithObject(name string)`                                             |
| Constructor          | `obj.WithConstructor(fn *dagger.Function)`                                          |
| Add function         | `obj.WithFunction(fn *dagger.Function)`                                             |
| Register module      | `dag.Module().WithObject(obj)`                                                      |
| Declare function     | `dag.Function(name string, returnType *dagger.TypeDef)`                             |
| Add arg              | `fn.WithArg(name string, typeDef *dagger.TypeDef, opts dagger.FunctionWithArgOpts)` |
| Arg description      | `dagger.FunctionWithArgOpts{Description: string}`                                   |
| Arg default          | `dagger.FunctionWithArgOpts{DefaultValue: dagger.JSON(val)}`                        |
| Function description | `fn.WithDescription(text string)`                                                   |
| Return void          | `dag.TypeDef().WithKind(dagger.TypeDefKindVoidKind)`                                |
| Return container     | `dag.TypeDef().WithObject("Container")`                                             |
| Return directory     | `dag.TypeDef().WithObject("Directory")`                                             |
| Return file          | `dag.TypeDef().WithObject("File")`                                                  |
| Optional type        | `typeDef.WithOptional(true)`                                                        |
| String type          | `dag.TypeDef().WithKind(dagger.TypeDefKindStringKind)`                              |

---

## Commands with no Dagger equivalent

These Earthly features are conceptually incompatible with Dagger's execution model.

| Earthly                                | Reason                                                                                  |
| -------------------------------------- | --------------------------------------------------------------------------------------- |
| `LOCALLY`                              | Runs on the host machine; Dagger modules are sandboxed                                  |
| `RUN --interactive`                    | Requires an interactive terminal; unavailable in Dagger modules                         |
| `SAVE IMAGE --cache-hint`              | Earthly-specific hint; Dagger uses content-addressed caching automatically              |
| `BUILD --auto-skip`                    | Earthly optimisation; Dagger's cache handles this natively                              |
| Earthly Cloud secrets (`+secrets/...`) | Requires Earthly Cloud account; can be bridged via `dag.SetSecret` if mapped explicitly |

---

## Unsupported Earthly commands (Earthly itself does not support these)

| Command            | Notes                                               |
| ------------------ | --------------------------------------------------- |
| `ADD <src> <dest>` | Extended COPY; marked not supported in Earthly docs |
| `SHELL [...]`      | Listed in Earthly docs as not supported             |
| `ONBUILD <cmd>`    | Dockerfile trigger; not supported by Earthly        |
| `STOPSIGNAL`       | Not supported by Earthly                            |
| `LOAD`             | Deprecated; superseded by `WITH DOCKER --load=...`  |
