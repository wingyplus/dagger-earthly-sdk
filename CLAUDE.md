# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Dagger Earthly SDK** enables running Earthly targets as native Dagger functions without requiring the Earthly CLI, Docker daemon, or BuildKit service. The SDK works as a [Dagger Runtime Module](https://docs.dagger.io/api/module-runtime) that parses Earthfile syntax and translates Earthfile instructions into Dagger API calls.

Example usage:
```bash
dagger init --sdk=github.com/wingyplus/dagger-earthly-sdk --source=. mymodule
dagger call echo-container --string-arg='Hello' file --path=/hello.txt contents
```

## Architecture

```
User Earthfile (text)
    ↓
earthfile.New() → Parse AST
    ↓
dagdag.ToModule() → Generate Dagger module schema (functions, arguments, return types)
    ↓
earthly.Interpreter → At runtime: walk AST statements, emit Dagger API calls
    ↓
*dagger.Container or void result
```

### Key Packages

| Package | Path | Purpose |
|---------|------|---------|
| `earthfile` | `runtime/earthfile/` | Parse Earthfile AST; map targets to Dagger functions; handle ARG scoping |
| `dagdag` | `runtime/dagdag/` | Convert parsed Earthfile into `*dagger.Module` definition for schema |
| `earthly` | `runtime/earthly/` | Interpreter: walk target recipe AST and emit native Dagger calls |

### Entry Points

- **Codegen** (`.dagger/main.go`): Implements `Codegen()` — reads Earthfile and generates schema at module initialization
- **Runtime** (`runtime/main.go`): Dispatches function calls via `CurrentFunctionCall()`
- **Templates** (`.dagger/templates/Earthfile`): Default Earthfile template shown to users on `dagger init`

## Development Commands

### Tests

```bash
# All unit tests
cd runtime && go test ./...

# Specific package with verbose output
cd runtime && go test ./earthly -v

# Single test
cd runtime && go test ./earthfile -run TestNew -v
```

### End-to-End

```bash
./scripts/init.sh simple
cd simple
dagger shell -c '. | echo-container "🌍🚀" | file /hello.txt | contents'
```

### Dagger Module Development

```bash
# Register local changes with the Dagger engine
dagger develop

# Regenerate Go SDK types if GraphQL schema changes
cd .dagger && go generate ./...
```

## Important Design Decisions

### ARG Scoping (VERSION 0.7+ with `--explicit-global`)

- `ARG NAME` in **base recipe**: scoped to base target only, NOT global
- `ARG --global NAME` in **base recipe**: visible in all targets, added as optional parameter to every Dagger function
- `ARG NAME` in **target recipe**: scoped to that target, mapped to a Dagger function parameter
- `ARG --required NAME`: becomes a required (non-optional) Dagger parameter

### Built-in ARG Filtering

These ARGs are parsed but excluded from Dagger function parameters: all `EARTHLY_*` names, `TARGETPLATFORM`, `TARGETOS`, `TARGETARCH`, `TARGETVARIANT`, and their native/user variants. See `runtime/earthfile/target.go` for the complete list.

### Naming Conventions

- Target names → Dagger function names: `strcase.ToCamel()` (`my-target` → `MyTarget`)
- ARG names → parameter names: `strcase.ToLowerCamel()` (`MY_ARG` → `myArg`)
- Comments above targets become function descriptions

### Return Types

- Targets with `SAVE IMAGE` → return `*dagger.Container`
- Targets without `SAVE IMAGE` → return `nil` (void/side-effect)
- Detection is static during codegen (even if `SAVE IMAGE` is inside `IF`)

## Implementation Status

**Implemented:** FROM, COPY, RUN (shell and exec forms), ARG, ENV, WORKDIR, USER, ENTRYPOINT, CMD, LABEL, EXPOSE, SAVE IMAGE, SAVE ARTIFACT (partial), `FROM +target` (recursive builds)

**Returns error:** `WITH DOCKER ... END` (requires Docker daemon, incompatible with Dagger sandbox)

**Silently skipped:** `IF`, `FOR`, `TRY/CATCH`, `WAIT`, `VOLUME`, `SHELL`, `STOPSIGNAL`, `HEALTHCHECK`

**Not yet implemented (returns error):** BUILD, FROM DOCKERFILE, GIT CLONE, DO, FUNCTION/COMMAND, IMPORT, LET/SET, CACHE, HOST, ADD, ONBUILD, LOAD, LOCALLY

See `docs/progress.md` for status details and `docs/earthly-dagger-mapping.md` for the Earthly → Dagger API reference.

## CI

GitHub Actions (`.github/workflows/test.yaml`):
- **runtime job**: `dagger run go test ./...` in `runtime/` on Go 1.24
- **e2e job**: bootstraps SDK via `./scripts/init.sh` and runs a smoke test

Dagger version pinned to `v0.18.8` in CI.
