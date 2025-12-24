package policy

import (
	"fmt"
	"reflect"
	"slices"
)

type Operator func(ctx RequestContext, args []any) (EvalResult, error)

var operators = make(map[string]Operator)

func init() {
	operators["And"] = opAnd
	operators["Or"] = opOr
	operators["Not"] = opNot
	operators["Eq"] = opEq
	operators["Contains"] = opContains
	operators["Load"] = opLoad
	// Additional operators can be registered here...
}

func opAnd(ctx RequestContext, args []any) (EvalResult, error) {

	for i, arg := range args {
		evaluated, ok := arg.(bool)
		if !ok {
			err := fmt.Errorf("bad argument type for AND at index %d. Expected bool but got %s\n", i, reflect.TypeOf(arg))
			return EvalResult{
				Operator: "And",
				Error:    err.Error(),
			}, err
		}

		if !evaluated {
			return EvalResult{
				Operator: "And",
				Result:   false,
			}, nil
		}
	}

	return EvalResult{
		Operator: "And",
		Result:   true,
	}, nil
}

func opOr(ctx RequestContext, args []any) (EvalResult, error) {
	for i, arg := range args {
		evaluated, ok := arg.(bool)
		if !ok {
			err := fmt.Errorf("bad argument type for OR at index %d. Expected bool but got %s\n", i, reflect.TypeOf(arg))
			return EvalResult{
				Operator: "Or",
				Error:    err.Error(),
			}, err
		}

		if evaluated {
			return EvalResult{
				Operator: "Or",
				Result:   true,
			}, nil
		}
	}

	return EvalResult{
		Operator: "Or",
		Result:   false,
	}, nil
}

func opNot(ctx RequestContext, args []any) (EvalResult, error) {
	if len(args) != 1 {
		err := fmt.Errorf("bad argument length for NOT. Expected 1 but got %d\n", len(args))
		return EvalResult{
			Operator: "Not",
			Error:    err.Error(),
		}, err
	}

	evaluated, ok := args[0].(bool)
	if !ok {
		err := fmt.Errorf("bad argument type for NOT. Expected bool but got %s\n", reflect.TypeOf(args[0]))
		return EvalResult{
			Operator: "Not",
			Error:    err.Error(),
		}, err
	}

	return EvalResult{
		Operator: "Not",
		Result:   !evaluated,
	}, nil
}

func opEq(ctx RequestContext, args []any) (EvalResult, error) {
	if len(args) != 2 {
		err := fmt.Errorf("bad argument length for EQ. Expected 2 but got %d\n", len(args))
		return EvalResult{
			Operator: "Eq",
			Error:    err.Error(),
		}, err
	}

	return EvalResult{
		Operator: "Eq",
		Result:   args[0] == args[1],
	}, nil
}

func opContains(ctx RequestContext, args []any) (EvalResult, error) {
	if len(args) != 2 {
		err := fmt.Errorf("bad argument length for CONTAINS. Expected 2 but got %d\n", len(args))
		return EvalResult{
			Operator: "Contains",
			Error:    err.Error(),
		}, err
	}

	arg0, ok := args[0].([]any)
	if !ok {
		err := fmt.Errorf("bad argument type for CONTAINS. Expected []any but got %s\n", reflect.TypeOf(args[0]))
		return EvalResult{
			Operator: "Contains",
			Error:    err.Error(),
		}, err
	}

	arg1 := args[1]

	return EvalResult{
		Operator: "Contains",
		Result:   slices.Contains(arg0, arg1),
	}, nil

}

func opLoad(ctx RequestContext, args []any) (EvalResult, error) {
	if len(args) != 1 {
		err := fmt.Errorf("bad argument length for Load. Expected 1 but got %d\n", len(args))
		return EvalResult{
			Operator: "Load",
			Error:    err.Error(),
		}, err
	}

	key, ok := args[0].(string)
	if !ok {
		err := fmt.Errorf("bad argument type for Load. Expected string but got %s\n", reflect.TypeOf(args[0]))
		return EvalResult{
			Operator: "Load",
			Error:    err.Error(),
		}, err
	}

	mappedCtx := structToMap(ctx)
	value, ok := resolveDotNotation(mappedCtx, key)
	if !ok {
		err := fmt.Errorf("key not found: %s\n", key)
		return EvalResult{
			Operator: "Load",
			Error:    err.Error(),
		}, err
	}

	return EvalResult{
		Operator: "Load",
		Result:   value,
	}, nil
}
