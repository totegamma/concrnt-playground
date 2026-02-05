package service

import (
	"context"

	"github.com/totegamma/concrnt-playground/policy"

	"github.com/totegamma/concrnt-playground/internal/domain"
)

type PolicyService struct {
	global policy.PolicyDocument
}

func NewPolicyService(global policy.PolicyDocument) *PolicyService {
	return &PolicyService{global: global}
}

func (s *PolicyService) Eval(ctx context.Context, req policy.RequestContext, stack [][]policy.PolicyDocument, action string) error {
	ctx, span := tracer.Start(ctx, "Policy.Service.Eval")
	defer span.End()

	stack = append([][]policy.PolicyDocument{{s.global}}, stack...)

	conclusion, error := policy.EvaluateStack(ctx, req, stack, action)
	if error != nil {
		return error
	}

	switch conclusion {
	case policy.ALLOW, policy.OK:
		return nil
	default:
		return domain.PermissionError{Reason: "action denied by policy"}
	}
}
