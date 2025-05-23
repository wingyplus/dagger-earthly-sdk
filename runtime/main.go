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
	"github.com/wingyplus/dagger-earthly-sdk/sdk/dagdag"
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
func invoke(ctx context.Context, ef *earthfile.Earthfile, parentJson []byte, parentName, fnName string, inputArgs map[string][]byte) (_ any, err error) {
	switch parentName {
	// Register module
	case "":
		return dagdag.ToModule(ef), nil
	case ef.ModuleName:
		switch fnName {
		// Constructor call
		case "":
			var parent earthly.Earthly
			if err := json.Unmarshal(parentJson, &parent); err != nil {
				panic(fmt.Errorf("failed to unmarshal parent object: %w", err))
			}
			if inputArgs["dockerUnixSock"] != nil {
				// HACK: Socket type doesn't expose `UnmarshalJSON` on https://github.com/dagger/dagger/blob/main/sdk/go/dagger.gen.go#L9342.
				var sockId string
				err := json.Unmarshal(inputArgs["dockerUnixSock"], &sockId)
				if err != nil {
					panic(fmt.Errorf("failed to unmarshal input arg dockerUnixSock: %w", err))
				}
				return earthly.New(dag.LoadSocketFromID(dagger.SocketID(sockId))), nil
			}
			return &parent, nil
		// Function call
		default:
			var parentArgs map[string]string
			if err := json.Unmarshal(parentJson, &parentArgs); err != nil {
				panic(fmt.Errorf("failed to unmarshal parent object: %w", err))
			}
			var dockerUnixSock *dagger.Socket
			if sockId, ok := parentArgs["DockerUnixSock"]; ok && sockId != "" {
				// HACK: Socket type doesn't expose `UnmarshalJSON` on https://github.com/dagger/dagger/blob/main/sdk/go/dagger.gen.go#L9342.
				dockerUnixSock = dag.LoadSocketFromID(dagger.SocketID(sockId))
			}
			parent := earthly.New(dockerUnixSock)

			target := ef.TargetFromFunctionName(fnName)
			if target == nil {
				return nil, fmt.Errorf("unknown function %s", fnName)
			}
			args, err := toEarthlyArgs(inputArgs)
			if err != nil {
				return nil, fmt.Errorf("cannot convert to earthly arguments: %w", err)
			}
			return parent.Invoke(
				ctx,
				dag.Host().Directory(ef.SourcePath),
				target,
				args,
			)
		}
	}

	panic("unreachable")
}

func toEarthlyArgs(inputArgs map[string][]byte) (args earthly.Args, err error) {
	args = make(earthly.Args)
	for k, v := range inputArgs {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, err
		}
		args[strcase.ToScreamingSnake(k)] = s
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

	inputArgs := map[string][]byte{}
	for _, fnArg := range fnArgs {
		argName, err := fnArg.Name(ctx)
		if err != nil {
			return fmt.Errorf("get fn arg name: %w", err)
		}
		argValue, err := fnArg.Value(ctx)
		if err != nil {
			return fmt.Errorf("get fn arg value: %w", err)
		}
		inputArgs[argName] = []byte(argValue)
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
