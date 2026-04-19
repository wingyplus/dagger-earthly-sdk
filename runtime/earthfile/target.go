package earthfile

import (
	"strings"

	"github.com/earthly/earthly/ast/spec"
	"github.com/iancoleman/strcase"
)

// builtinArgNames is the set of Earthly built-in ARG names that are populated
// by the Earthly runtime and must not be exposed as Dagger function parameters.
// Declaring one of these in an Earthfile brings it into scope, but its value
// can never be overridden by the user.
var builtinArgNames = map[string]bool{
	// General
	"EARTHLY_CI":        true,
	"EARTHLY_BUILD_SHA": true,
	"EARTHLY_LOCALLY":   true,
	"EARTHLY_PUSH":      true,
	"EARTHLY_VERSION":   true,
	// Target-related
	"EARTHLY_TARGET":                true,
	"EARTHLY_TARGET_NAME":           true,
	"EARTHLY_TARGET_PROJECT":        true,
	"EARTHLY_TARGET_PROJECT_NO_TAG": true,
	"EARTHLY_TARGET_TAG":            true,
	"EARTHLY_TARGET_TAG_DOCKER":     true,
	// Git-related
	"EARTHLY_GIT_HASH":                    true,
	"EARTHLY_GIT_SHORT_HASH":              true,
	"EARTHLY_GIT_BRANCH":                  true,
	"EARTHLY_GIT_ORIGIN_URL":              true,
	"EARTHLY_GIT_PROJECT_NAME":            true,
	"EARTHLY_GIT_COMMIT_TIMESTAMP":        true,
	"EARTHLY_GIT_COMMIT_AUTHOR_TIMESTAMP": true,
	"EARTHLY_GIT_AUTHOR":                  true,
	"EARTHLY_GIT_AUTHOR_EMAIL":            true,
	"EARTHLY_GIT_AUTHOR_NAME":             true,
	"EARTHLY_GIT_CO_AUTHORS":              true,
	"EARTHLY_GIT_REFS":                    true,
	"EARTHLY_SOURCE_DATE_EPOCH":           true,
	// Platform-related
	"TARGETPLATFORM": true,
	"TARGETOS":       true,
	"TARGETARCH":     true,
	"TARGETVARIANT":  true,
	"NATIVEPLATFORM": true,
	"NATIVEOS":       true,
	"NATIVEARCH":     true,
	"NATIVEVARIANT":  true,
	"USERPLATFORM":   true,
	"USEROS":         true,
	"USERARCH":       true,
	"USERVARIANT":    true,
}

// IsBuiltinArg reports whether name is an Earthly built-in ARG. Built-in ARGs
// are populated by the runtime and must not be exposed as Dagger function
// parameters. The EARTHLY_ prefix check future-proofs against new additions
// not yet in the static table.
func IsBuiltinArg(name string) bool {
	if builtinArgNames[name] {
		return true
	}
	return strings.HasPrefix(name, "EARTHLY_")
}

type ArgOpt struct {
	Name         string
	DefaultValue string
	Required     bool
	Global       bool
	Doc          string
}

type Target struct {
	Name     string
	Doc      string
	Args     map[string]ArgOpt
	ArgOrder []string // declaration order of ARG names
	Ast      spec.Target
	// TODO: sourcemap
}

// Output returns an image output specified in `SAVE IMAGE` instruction.
func (t *Target) Output() (string, bool) {
	for _, statement := range t.Ast.Recipe {
		if cmd := statement.Command; cmd != nil {
			if out, found := saveImageOutput(cmd); found {
				return out, found
			}
		}

		if stmt := statement.If; stmt != nil {
			for _, statement := range stmt.IfBody {
				if statement.Command != nil {
					if out, found := saveImageOutput(statement.Command); found {
						return out, found
					}
				}
			}
			if stmt.ElseBody != nil {
				for _, statement := range *stmt.ElseBody {
					if statement.Command != nil {
						if out, found := saveImageOutput(statement.Command); found {
							return out, found
						}
					}
				}
			}
		}
	}
	return "", false
}

func saveImageOutput(cmd *spec.Command) (string, bool) {
	if cmd.Name == "SAVE IMAGE" {
		// SAVE IMAGE with no arguments is valid Earthly syntax — the image is
		// saved without an explicit tag. Treat it as a container-returning target.
		if len(cmd.Args) == 0 {
			return "", true
		}
		if cmd.Args[0] == "--push" {
			return cmd.Args[1], true
		}
		return cmd.Args[0], true
	}
	return "", false
}

func parseTarget(ast spec.Target) *Target {
	target := &Target{
		Name: ast.Name,
		Doc:  strings.TrimSpace(ast.Docs),
		Args: map[string]ArgOpt{},
		Ast:  ast,
	}
	for _, statement := range ast.Recipe {
		if cmd := statement.Command; cmd != nil {
			if cmd.Name == "ARG" {
				arg := parseArg(cmd.Args)
				arg.Doc = strings.TrimSpace(cmd.Docs)
				if _, exists := target.Args[arg.Name]; !exists {
					target.ArgOrder = append(target.ArgOrder, arg.Name)
				}
				target.Args[arg.Name] = arg
			}
		}
	}
	return target
}

type TargetsMap map[string]*Target

func parseTargetsMap(asts []spec.Target) (targets TargetsMap) {
	targets = make(TargetsMap)
	for _, ast := range asts {
		target := parseTarget(ast)
		targets[strcase.ToCamel(target.Name)] = target
	}
	return
}

// parseArg parses the argument tokens from an ARG command. It handles flags
// (--required, --global) in any order before the name token.
//
// Token forms accepted:
//
//	[--required] [--global] name
//	[--required] [--global] name = value
func parseArg(arg []string) (opt ArgOpt) {
	// Consume leading flags in any order.
	for len(arg) > 0 && strings.HasPrefix(arg[0], "--") {
		switch arg[0] {
		case "--required":
			opt.Required = true
		case "--global":
			opt.Global = true
		}
		arg = arg[1:]
	}
	if len(arg) == 0 {
		return
	}
	opt.Name = arg[0]
	// Tokens: name "=" value  →  len == 3, value at index 2.
	if len(arg) > 2 {
		opt.DefaultValue = strings.Trim(arg[2], `"`)
	}
	return
}
