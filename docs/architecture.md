# Architecture

The SDK works as a [Dagger Runtime Module](https://docs.dagger.io/api/module-runtime). When you initialize a project with this SDK, Dagger calls two entry points:

- **Codegen** (`.dagger/main.go`) — reads your `Earthfile` and generates the Dagger module schema (function names, argument types, return types).
- **Runtime** (`runtime/`) — when a user calls a generated function, the runtime walks the Earthfile AST and executes each instruction as native Dagger API calls.

```
Earthfile (text)
    └─ earthfile.New()          parse AST
         └─ dagdag.ToModule()   generate Dagger module schema
              └─ Interpreter    translate instructions to Dagger calls at runtime
```

## Key packages

| Package     | Path                 | Purpose                                                     |
| ----------- | -------------------- | ----------------------------------------------------------- |
| `earthfile` | `runtime/earthfile/` | Parse Earthfile AST; map targets to Dagger functions        |
| `dagdag`    | `runtime/dagdag/`    | Convert parsed Earthfile into a `*dagger.Module` definition |
| `earthly`   | `runtime/earthly/`   | Interpreter: walk AST and emit native Dagger calls          |
