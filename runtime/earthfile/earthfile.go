package earthfile

import (
	"context"
	"strings"

	"github.com/earthly/earthly/ast"
	"github.com/earthly/earthly/ast/spec"
)

type NamedReader = ast.NamedReader

var FromReader = ast.FromReader

type Earthfile struct {
	Ast        spec.Earthfile
	ModuleName string
	Targets    TargetsMap
	// GlobalArgs holds ARGs declared with --global in the base recipe.
	// These are visible to every target in the Earthfile and are exposed as
	// optional parameters on every Dagger function.
	GlobalArgs map[string]ArgOpt
	SourcePath string
}

// Initiate Earthfile from path.
func New(ctx context.Context, path string, modname string) (*Earthfile, error) {
	return NewFromOpts(ctx, path, ast.FromPath(path+"/Earthfile"), modname)
}

func NewFromOpts(ctx context.Context, path string, opt ast.FromOpt, modname string) (*Earthfile, error) {
	ast, err := ast.ParseOpts(ctx, opt, ast.WithSourceMap())
	if err != nil {
		return nil, err
	}
	return &Earthfile{
		Ast:        ast,
		ModuleName: modname,
		Targets:    parseTargetsMap(ast.Targets),
		GlobalArgs: parseGlobalArgs(ast.BaseRecipe),
		SourcePath: path,
	}, nil
}

func (ef *Earthfile) TargetFromFunctionName(name string) *Target {
	return ef.Targets[name]
}

// parseGlobalArgs scans the base recipe for ARG --global declarations and
// returns them keyed by name. Plain ARGs in the base recipe (without --global)
// are ignored — the SDK targets VERSION 0.7+ semantics where the explicit-global
// feature flag is on by default.
func parseGlobalArgs(base spec.Block) map[string]ArgOpt {
	globals := make(map[string]ArgOpt)
	for _, stmt := range base {
		if cmd := stmt.Command; cmd != nil && cmd.Name == "ARG" {
			arg := parseArg(cmd.Args)
			arg.Doc = strings.TrimSpace(cmd.Docs)
			if arg.Global {
				globals[arg.Name] = arg
			}
		}
	}
	return globals
}
