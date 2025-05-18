package earthfile

import (
	"context"

	"github.com/earthly/earthly/ast"
	"github.com/earthly/earthly/ast/spec"
)

type NamedReader = ast.NamedReader

var FromReader = ast.FromReader

type Earthfile struct {
	Ast        spec.Earthfile
	ModuleName string
	Targets    TargetsMap
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
		SourcePath: path,
	}, nil
}

func (ef *Earthfile) TargetFromFunctionName(name string) *Target {
	return ef.Targets[name]
}
