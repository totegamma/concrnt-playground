package service

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/policy"
)

type PolicyService struct {
	global policy.PolicyDocument
	cache  *cache.Cache
}

func NewPolicyService(global policy.PolicyDocument) *PolicyService {
	return &PolicyService{
		global: global,
		cache:  cache.New(10*time.Minute, 15*time.Minute),
	}
}

func (s *PolicyService) ResolvePolicyURL(ctx context.Context, policyURL string) (policy.PolicyDocument, error) {
	ctx, span := tracer.Start(ctx, "Policy.Service.ResolvePolicyURL")
	defer span.End()

	cached, found := s.cache.Get(policyURL)
	if found {
		return cached.(policy.PolicyDocument), nil
	}

	resp, err := http.Get(policyURL)
	if err != nil {
		return policy.PolicyDocument{}, err
	}
	defer resp.Body.Close()

	var policyDoc policy.PolicyDocument
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&policyDoc)
	if err != nil {
		return policy.PolicyDocument{}, err
	}

	s.cache.Set(policyURL, policyDoc, cache.DefaultExpiration)

	return policyDoc, nil
}

func (s *PolicyService) Eval(ctx context.Context, req policy.RequestContext, stack [][]concrnt.Policy, action string) error {
	ctx, span := tracer.Start(ctx, "Policy.Service.Eval")
	defer span.End()

	var policyStack policy.PolicyStack
	policyStack = append(policyStack, []policy.EvaluationSet{
		{
			PolicyDocument: s.global,
		},
	})

	for _, layer := range stack {
		policyLayer := []policy.EvaluationSet{}
		for _, p := range layer {
			doc, err := s.ResolvePolicyURL(ctx, p.URL)
			if err != nil {
				continue // TODO:
			}

			policyLayer = append(policyLayer, policy.EvaluationSet{
				PolicyDocument: doc,
				Params:         p.Params,
				Defaults:       p.Defaults,
			})
		}
		policyStack = append(policyStack, policyLayer)
	}

	conclusion, error := policy.EvaluateStack(ctx, req, policyStack, action)
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
