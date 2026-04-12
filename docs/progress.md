# Implementation Progress

This document tracks the current implementation status of the Dagger Earthly SDK — specifically the native Dagger interpreter that translates Earthfile syntax into Dagger API calls without spawning the Earthly CLI.

See [architecture.md](architecture.md) for an overview of how the SDK is structured.

## What works today

### Earthfile instructions

| Instruction                  | Status      | Notes                                                                                                   |
| ---------------------------- | ----------- | ------------------------------------------------------------------------------------------------------- |
| `FROM <image>`               | Implemented | Translates to `dag.Container().From(image)`                                                             |
| `FROM +target`               | Implemented | Recursively builds the local target and inherits its container                                          |
| `RUN <cmd>`                  | Implemented | Shell form (`sh -c`) and exec form both supported                                                       |
| `COPY <src> <dest>`          | Implemented | Local build context paths (file and directory)                                                          |
| `COPY +target/<path> <dest>` | Implemented | Cross-target artifact extraction                                                                        |
| `ARG NAME`                   | Implemented | Optional ARG; exposed as optional Dagger function argument                                              |
| `ARG --required NAME`        | Implemented | Required ARG; exposed as required Dagger function argument                                              |
| `ARG NAME=default`           | Implemented | Default value used when caller omits the argument                                                       |
| `ENV KEY=VALUE`              | Implemented | Translates to `Container.WithEnvVariable()`                                                             |
| `ENV KEY VALUE`              | Implemented | Both `KEY=VALUE` and `KEY VALUE` forms accepted                                                         |
| `WORKDIR <path>`             | Implemented | Translates to `Container.WithWorkdir()`                                                                 |
| `USER <user>`                | Implemented | Translates to `Container.WithUser()`                                                                    |
| `LABEL <key>=<value>`        | Implemented | Translates to `Container.WithLabel()`                                                                   |
| `ENTRYPOINT [...]`           | Implemented | Translates to `Container.WithEntrypoint()`                                                              |
| `CMD [...]`                  | Implemented | Translates to `Container.WithDefaultArgs()`                                                             |
| `SAVE IMAGE <name>`          | Implemented | Target returns `*dagger.Container` to the caller                                                        |
| `SAVE IMAGE --push <name>`   | Implemented | Same as `SAVE IMAGE`; actual push deferred to caller                                                    |
| `SAVE ARTIFACT <path>`       | Partial     | Path is recorded; artifact is accessible via the returned container — `AS LOCAL` export not implemented |

### Module schema generation

| Feature                                            | Status      | Notes                                                        |
| -------------------------------------------------- | ----------- | ------------------------------------------------------------ |
| Target → Dagger function                           | Implemented | Each Earthfile target becomes a callable Dagger function     |
| `ARG` → function argument                          | Implemented | Type is always `String`; optional/required/default preserved |
| `SAVE IMAGE` → `Container` return type             | Implemented | Targets with `SAVE IMAGE` return `*dagger.Container`         |
| `SAVE IMAGE` inside `IF` → `Container` return type | Implemented | Detected statically during codegen                           |
| No `SAVE IMAGE` → `Void` return type               | Implemented | Side-effect targets return nothing                           |
| Target doc comment → function description          | Implemented | Leading `#` comments above a target are propagated           |

## What is not yet implemented

### Control flow statements

These Earthfile statement types exist in the AST (`spec.Statement`) but are silently skipped by the interpreter today.

| Statement                   | Behaviour today                             | Path to implement                                                                                          |
| --------------------------- | ------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `IF ... END`                | Silently skipped; container state unchanged | Run the test expression with `Container.WithExec().ExitCode(ctx)`, then evaluate the matching branch block |
| `ELSE IF` / `ELSE`          | Silently skipped                            | Part of the `IF` implementation above                                                                      |
| `FOR x IN ... END`          | Silently skipped                            | Evaluate the list expression via `Stdout(ctx)`, split, and iterate sequentially                            |
| `TRY ... CATCH ... FINALLY` | Silently skipped                            | Wrap the try-block evaluation in a Go error handler                                                        |
| `WAIT ... END`              | Silently skipped                            | Force `Sync(ctx)` on all containers produced inside the block                                              |

### Commands that return an error

These commands are actively rejected with an error message.

| Command               | Behaviour today  | Path to implement                                                                 |
| --------------------- | ---------------- | --------------------------------------------------------------------------------- |
| `WITH DOCKER ... END` | Returns an error | Spin a `docker:dind` service bound to the exec container for the block's duration |

### Commands silently skipped

These commands are accepted but their effect is not applied. This is intentional for the first pass — they are metadata that either has no Dagger equivalent or is not needed for basic functionality.

| Command         | Why skipped                             | Notes                                                                          |
| --------------- | --------------------------------------- | ------------------------------------------------------------------------------ |
| `EXPOSE <port>` | No Dagger equivalent for image metadata | Dagger exposes ports via `Container.WithExposedPort()` on services, not images |
| `VOLUME <path>` | No exact Dagger equivalent              | Could be approximated with `WithMountedTemp()`                                 |
| `SHELL [...]`   | Interpreter state only                  | Would change the shell wrapper used for subsequent `RUN` commands              |
| `STOPSIGNAL`    | No Dagger SDK API                       | OCI stop signal metadata not exposed in the Go SDK                             |
| `HEALTHCHECK`   | No Dagger SDK API                       | OCI healthcheck metadata not exposed in the Go SDK                             |

### Commands not yet handled

These commands will return an "unsupported Earthly command" error if encountered.

| Command                             | Notes                                                    |
| ----------------------------------- | -------------------------------------------------------- |
| `BUILD +target [--build-arg k=v]`   | Invoke another target as a side-effect (no return value) |
| `FROM DOCKERFILE [--build-arg] ...` | Use `Directory.DockerBuild()` in Dagger                  |
| `GIT CLONE <url> <dest>`            | Use `dag.Git(url).Branch(ref).Tree()`                    |
| `DO +FUNCTION`                      | Inline-expand a `FUNCTION` block into the current target |
| `COMMAND` / `FUNCTION`              | Define a reusable block (user-defined command)           |
| `IMPORT <ref> [AS <alias>]`         | Alias for a remote Earthfile reference                   |
| `LET <name>=<value>`                | Mutable local variable (distinct from `ARG`)             |
| `SET <name>=<value>`                | Reassign a `LET` variable                                |
| `CACHE <path>`                      | Mount a persistent cache volume for a path               |
| `HOST <name> <ip>`                  | Inject a `/etc/hosts` entry                              |
| `ADD <src> <dest>`                  | Extended `COPY` with URL and tar-extraction support      |
| `ONBUILD <cmd>`                     | Trigger instruction for downstream images                |
| `PROJECT <org>/<name>`              | Earthly Cloud project binding                            |
| `LOAD`                              | Load a Docker image into the build context               |

### Features with no Dagger equivalent

These Earthly features are conceptually incompatible with Dagger's execution model and are unlikely to be implementable natively.

| Feature                                | Reason                                                                                                     |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `LOCALLY`                              | Runs commands on the host machine. Dagger modules are sandboxed and cannot run arbitrary host commands.    |
| `RUN --interactive`                    | Interactive shell debugging. Dagger has no interactive terminal in modules.                                |
| `SAVE IMAGE --cache-hint`              | Earthly-specific cache hint. Dagger uses its own content-addressed cache automatically.                    |
| `--auto-skip` on `BUILD`               | Earthly-specific build skip optimisation. Dagger's cache handles this natively.                            |
| Earthly Cloud secrets (`+secrets/...`) | Requires an Earthly Cloud account and satellite. Could be bridged via Dagger secrets if mapped explicitly. |
| Earthly Cloud satellites               | Remote build runners specific to the Earthly platform.                                                     |

### RUN flags not yet handled

The `RUN` command accepts flags that are currently stripped silently.

| Flag                                | Notes                                                                                |
| ----------------------------------- | ------------------------------------------------------------------------------------ |
| `--mount=type=cache,target=<path>`  | Map to `Container.WithMountedCache()`                                                |
| `--mount=type=secret,target=<path>` | Map to `Container.WithMountedSecret()`                                               |
| `--secret <name>=<ref>`             | Map to `Container.WithSecretVariable()`                                              |
| `--ssh`                             | Map to `Container.WithUnixSocket()` for the SSH agent socket                         |
| `--push`                            | Deferred effect — run only if the overall build succeeds and `--push` mode is active |
| `--no-cache`                        | No direct Dagger equivalent; Dagger caches all layers by default                     |
| `--privileged`                      | Map to `ContainerWithExecOpts{InsecureRootCapabilities: true}`                       |
| `--allow-privileged`                | Same as above                                                                        |
| `--network=none`                    | No direct Dagger SDK API                                                             |

### COPY flags not yet handled

| Flag                              | Notes                                                               |
| --------------------------------- | ------------------------------------------------------------------- |
| `--dir`                           | Force directory semantics even for non-`/`-suffixed paths           |
| `--chown <user>`                  | Set ownership on copied files                                       |
| `--chmod <mode>`                  | Set permissions on copied files                                     |
| `--keep-ts`                       | Preserve source file timestamps                                     |
| `--from=<image>`                  | Copy from a foreign image (`COPY --from=image src dest`)            |
| Build args on `COPY +target/path` | `COPY +t/a dest --build-arg KEY=VAL` — build args not forwarded yet |

## Known limitations

- **`looksLikeDir` heuristic** — the interpreter decides whether a COPY source is a file or directory by checking for a dot in the final path component. This is a heuristic and will mis-classify paths like `/etc/hosts` (treated as file) or `/dist/v1` (treated as directory). A robust implementation would stat the path inside the container.

- **ARG scope** — ARGs declared in the middle of a recipe are merged into a single flat map at the start. In Earthly, an ARG declared after a `RUN` is only in scope for subsequent instructions. The current implementation does not model this ordering.

- **Cross-target build args** — `FROM +base --BUILD_ARG=value` and `COPY +t/path dest --build-arg KEY=VAL` do not forward build args to the referenced target today. The referenced target always uses its own defaults.

- **Target cache key** — the memoization cache in `Interpreter` is keyed by target name only. Two calls to the same target with different args will return the first result. This matters for targets called with different `--build-arg` values from multiple callers.
