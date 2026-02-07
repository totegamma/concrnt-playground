package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/policy"
)

type PolicyService struct {
	global policy.Policy
	cache  *cache.Cache
}

func NewPolicyService(global policy.Policy) *PolicyService {
	return &PolicyService{
		global: global,
		cache:  cache.New(10*time.Minute, 15*time.Minute),
	}
}

func (s *PolicyService) ResolvePolicyURL(ctx context.Context, policyURL string) (policy.Policy, error) {
	ctx, span := tracer.Start(ctx, "Policy.Service.ResolvePolicyURL")
	defer span.End()

	cached, found := s.cache.Get(policyURL)
	if found {
		return cached.(policy.Policy), nil
	}

	resp, err := http.Get(policyURL)
	if err != nil {
		span.RecordError(err)
		return policy.Policy{}, err
	}
	defer resp.Body.Close()

	var policyDoc policy.PolicyDocument
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&policyDoc)
	if err != nil {
		span.RecordError(err)
		return policy.Policy{}, err
	}

	policyAny, ok := policyDoc.Versions["2025-12-23"]
	if !ok {
		return policy.Policy{}, fmt.Errorf("unsupported policy version in %s", policyURL)
	}

	policyString, err := json.Marshal(policyAny)
	if err != nil {
		span.RecordError(err)
		return policy.Policy{}, err
	}

	var policy20251223 policy.Policy
	err = json.Unmarshal(policyString, &policy20251223)
	if err != nil {
		span.RecordError(err)
		return policy.Policy{}, err
	}

	s.cache.Set(policyURL, policy20251223, cache.DefaultExpiration)

	return policy20251223, nil
}

func (s *PolicyService) Eval(ctx context.Context, req policy.RequestContext, stack [][]concrnt.Policy, action string) error {
	ctx, span := tracer.Start(ctx, "Policy.Service.Eval")
	defer span.End()

	var policyStack policy.PolicyStack
	policyStack = append(policyStack, []policy.EvaluationSet{
		{
			Policy: s.global,
		},
	})

	for _, layer := range stack {
		policyLayer := []policy.EvaluationSet{}
		for _, p := range layer {
			pol, err := s.ResolvePolicyURL(ctx, p.URL)
			if err != nil {
				span.RecordError(err)
				continue // TODO:
			}

			policyLayer = append(policyLayer, policy.EvaluationSet{
				Policy:   pol,
				Params:   p.Params,
				Defaults: p.Defaults,
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
