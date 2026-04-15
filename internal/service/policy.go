package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/policy"
)

type PolicyService struct {
	global policy.Policy
	client *client.Client
	cache  *cache.Cache
}

func NewPolicyService(global policy.Policy, client *client.Client) *PolicyService {
	return &PolicyService{
		global: global,
		client: client,
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

func (s *PolicyService) resolvePolicyStack(ctx context.Context, stack [][]concrnt.Policy) (policy.PolicyStack, error) {
	ctx, span := tracer.Start(ctx, "Policy.Service.resolvePolicyStack")
	defer span.End()

	result := policy.PolicyStack{}

	for _, layer := range stack {
		policyLayer := []policy.EvaluationSet{}
		for _, p := range layer {

			if p.URL != nil {
				pol, err := s.ResolvePolicyURL(ctx, *p.URL)
				if err != nil {
					span.RecordError(err)
					continue
				}

				policyLayer = append(policyLayer, policy.EvaluationSet{
					Policy:   pol,
					Params:   p.Params,
					Defaults: p.Defaults,
				})
			}

			if p.Ref != nil {
				var doc concrnt.Document[any]
				err := s.client.GetRecord(ctx, *p.Ref, nil, &doc)
				if err != nil {
					span.RecordError(err)
					continue
				}

				if doc.Policies == nil {
					continue
				}

				refRawStack := [][]concrnt.Policy{*doc.Policies}
				refPolicyStack, err := s.resolvePolicyStack(ctx, refRawStack)
				if err != nil {
					span.RecordError(err)
					continue
				}

				if len(refPolicyStack) != 1 {
					fmt.Println("PROGRAM ERROR: invalid policy stack in reference", *p.Ref)
					span.RecordError(fmt.Errorf("invalid policy stack in reference %s", *p.Ref))
					continue
				}
				policyLayer = append(policyLayer, refPolicyStack[0]...)
			}

		}
		result = append(result, policyLayer)
	}

	return result, nil
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

	additionalStack, err := s.resolvePolicyStack(ctx, stack)
	if err != nil {
		span.RecordError(err)
		return err
	}

	policyStack = append(policyStack, additionalStack...)

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
