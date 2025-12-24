package policy

import (
	"fmt"
	"testing"
)

func TestPolicy(t *testing.T) {

	ctx := RequestContext{
		Params: map[string]any{
			"user": "alice",
			"role": "admin",
		},
	}

	expr := Expr{
		Operator: "Eq",
		Args: []Expr{
			{
				Operator: "Load",
				Args: []Expr{
					{
						Const: "params.role",
					},
				},
			},
			{
				Const: "admin",
			},
		},
	}

	result, err := Eval(ctx, expr)
	if err != nil {
		fmt.Println("Eval error:", err)
		t.Fatalf("Eval failed: %v", err)
	}

	fmt.Println("Eval result:", result.Result)

	t.Fail()
}
