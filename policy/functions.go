package policy

import (
	"fmt"
)

func SummerizeConclusion(conclusions []Conclusion, defaultAllow bool) bool {
	result := UNSET
	for _, c := range conclusions {
		switch c {
		case ALLOW:
			return true
		case DENY:
			return false
		default:
			result = result.Or(c)
		}
	}
	if result == UNSET {
		return defaultAllow
	}
	return result == ALLOW
}

func EvaluatePolicy(policydoc PolicyDocument, ctx RequestContext, action string) (Conclusion, error) {

	policy, ok := policydoc.Versions["2024-01-01"]
	if !ok {
		return UNSET, fmt.Errorf("unsupported policy version")
	}

	statements, ok := policy.Statements[action]
	if !ok {
		// No statements for this action
		return UNSET, nil
	}

	conclusion := UNSET
	for _, stmt := range statements {
		evalResult, err := Eval(ctx, stmt.Condition)
		if err != nil {
			continue
		}

		if evalResult.Result == true {
			emit := ParseConclusion(stmt.Emit)
			conclusion = conclusion.Or(emit)
		}
	}
	return conclusion, nil
}

func Eval(ctx RequestContext, expr Expr) (EvalResult, error) {

	if expr.Const != nil {
		return EvalResult{
			Operator: "Const",
			Result:   expr.Const,
		}, nil
	}

	args := make([]any, 0, len(expr.Args))
	for _, arg := range expr.Args {
		result, err := Eval(ctx, arg)
		if err != nil {
			return EvalResult{
				Operator: expr.Operator,
				Error:    err.Error(),
			}, err
		}
		args = append(args, result.Result)
	}

	if operatorFunc, exists := operators[expr.Operator]; exists {
		return operatorFunc(ctx, args)
	}

	err := fmt.Errorf("unknown operator: %s\n", expr.Operator)
	return EvalResult{
		Operator: expr.Operator,
		Error:    err.Error(),
	}, err
}
