package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"dagger.io/dagger"
	"dagger.io/dagger/dag"
	"github.com/iancoleman/strcase"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthfile"
	"github.com/wingyplus/dagger-earthly-sdk/sdk/earthly"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()
	defer dag.Close()

	modname, err := dag.CurrentModule().Name(ctx)
	if err != nil {
		fmt.Printf("current module: %v\n", err)
		os.Exit(2)
	}

	path := os.Args[1]
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("args: %v\n", err)
		os.Exit(2)
	}

	ef, err := earthfile.New(ctx, path, modname)
	if err != nil {
		fmt.Printf("earthfile: %v\n", err)
		os.Exit(2)
	}

	if err := dispatch(ctx, ef); err != nil {
		fmt.Printf("dispatch: %v\n", err)
		os.Exit(2)
	}
}

// TODO: implements invoke target.
func invoke(ctx context.Context, ef *earthfile.Earthfile, parentJson []byte, parentName, fnName string, inputArgs map[string]string) (_ any, err error) {
	// TODO: use me.
	_ = parentJson
	switch parentName {
	case "":
		return ef.ToModule(), nil
	case ef.ModuleName:
		target := ef.TargetFromFunctionName(fnName)
		args := toEarthlyArgs(inputArgs)
		return earthly.New(nil).Invoke(
			ctx,
			dag.Host().Directory(ef.SourcePath),
			target,
			args,
		)
	default:
		panic("unreachable")
	}
}

func toEarthlyArgs(inputArgs map[string]string) (args earthly.Args) {
	args = make(earthly.Args)
	for k, v := range inputArgs {
		args[strcase.ToScreamingSnake(k)] = v
	}
	return
}

func dispatch(ctx context.Context, ef *earthfile.Earthfile) (rerr error) {
	fnCall := dag.CurrentFunctionCall()
	defer func() {
		if rerr != nil {
			if err := fnCall.ReturnError(ctx, convertError(rerr)); err != nil {
				fmt.Println("failed to return error:", err, "\noriginal error:", rerr)
			}
		}
	}()

	parentName, err := fnCall.ParentName(ctx)
	if err != nil {
		return fmt.Errorf("get parent name: %w", err)
	}
	fnName, err := fnCall.Name(ctx)
	if err != nil {
		return fmt.Errorf("get fn name: %w", err)
	}
	parentJson, err := fnCall.Parent(ctx)
	if err != nil {
		return fmt.Errorf("get fn parent: %w", err)
	}
	fnArgs, err := fnCall.InputArgs(ctx)
	if err != nil {
		return fmt.Errorf("get fn args: %w", err)
	}

	inputArgs := map[string]string{}
	for _, fnArg := range fnArgs {
		argName, err := fnArg.Name(ctx)
		if err != nil {
			return fmt.Errorf("get fn arg name: %w", err)
		}
		argValue, err := fnArg.Value(ctx)
		if err != nil {
			return fmt.Errorf("get fn arg value: %w", err)
		}
		var value string
		if err := json.Unmarshal([]byte(argValue), &value); err != nil {
			return fmt.Errorf("unmarshal arg value: %w", err)
		}
		inputArgs[argName] = value
	}

	result, err := invoke(ctx, ef, []byte(parentJson), parentName, fnName, inputArgs)
	if err != nil {
		var exec *dagger.ExecError
		if errors.As(err, &exec) {
			return exec.Unwrap()
		}
		return err
	}
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := fnCall.ReturnValue(ctx, dagger.JSON(resultBytes)); err != nil {
		return fmt.Errorf("store return value: %w", err)
	}
	return nil
}

func convertError(rerr error) *dagger.Error {
	var gqlErr *gqlerror.Error
	if errors.As(rerr, &gqlErr) {
		dagErr := dag.Error(gqlErr.Message)
		if gqlErr.Extensions != nil {
			keys := make([]string, 0, len(gqlErr.Extensions))
			for k := range gqlErr.Extensions {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				val, err := json.Marshal(gqlErr.Extensions[k])
				if err != nil {
					fmt.Println("failed to marshal error value:", err)
				}
				dagErr = dagErr.WithValue(k, dagger.JSON(val))
			}
		}
		return dagErr
	}
	return dag.Error(rerr.Error())
}
