package policy

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

func EvaluatePolicy(ctx context.Context, policydoc PolicyDocument, req RequestContext, action string) (Conclusion, error) {
	ctx, span := tracer.Start(ctx, "Policy.EvaluatePolicy")
	defer span.End()

	policy, ok := policydoc.Versions["2025-12-23"]
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
		evalResult, err := Eval(req, stmt.Condition)
		if err != nil {
			span.RecordError(err)
			continue
		}

		resultJson, _ := json.MarshalIndent(evalResult, "", "  ")

		span.AddEvent("Evaluated Condition", trace.WithAttributes(
			attribute.String("action", action),
			attribute.String("condition", string(resultJson)),
		))

		if evalResult.Result == true {
			conclusion = conclusion.Or(stmt.Emit)
		}

	}

	if conclusion == UNSET {
		def := policy.Defaults[action]
		if def == "" {
			return DENY, nil
		}
		return def, nil
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
	evalResults := make([]EvalResult, 0, len(expr.Args))
	for _, arg := range expr.Args {
		result, err := Eval(ctx, arg)
		if err != nil {
			return EvalResult{
				Operator: expr.Operator,
				Error:    err.Error(),
			}, err
		}
		args = append(args, result.Result)
		evalResults = append(evalResults, result)
	}

	if operatorFunc, exists := operators[expr.Operator]; exists {
		result, err := operatorFunc(ctx, args)
		result.Args = evalResults
		return result, err
	}

	err := fmt.Errorf("unknown operator: %s\n", expr.Operator)
	return EvalResult{
		Operator: expr.Operator,
		Error:    err.Error(),
		Args:     evalResults,
	}, err
}
