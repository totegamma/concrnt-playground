package main

import (
	"encoding/json"

	"github.com/totegamma/concrnt-playground/policy"
)

var globalPolicyJson = `
{
	"name": "default",
	"versions": {
		"2025-12-23": {
			"statements": {
				"net.concrnt.core.commit.delete": [
					{
						"emit": "allow",
						"condition": {
							"op": "Eq",
							"args": [
								{
									"op": "Load",
									"args": [ {"const": "requester.ccid"} ]
								},
								{
									"op": "Load",
									"args": [ {"const": "this.author"} ]
								}
							]
						}
					}
				],
				"net.concrnt.core.resolve": [
					{
						"emit": "allow",
						"condition": {
							"op": "Or",
							"args": [
								{
									"op": "Eq",
									"args": [
										{
											"op": "Load",
											"args": [ {"const": "requester.ccid"} ]
										},
										{
											"op": "Load",
											"args": [ {"const": "this.author"} ]
										}
									]
								},
								{
									"op": "Eq",
									"args": [
										{
											"op": "Load",
											"args": [ {"const": "requester.ccid"} ]
										},
										{
											"op": "Load",
											"args": [ {"const": "this.owner"} ]
										}
									]
								}
							]
						}
					}
				]
			},
			"defaults": {
				"commit.delete": "deny"
			}
		}
	}
}`

func GetGlobalPolicy() policy.PolicyDocument {
	globalPolicy := policy.PolicyDocument{}
	err := json.Unmarshal([]byte(globalPolicyJson), &globalPolicy)
	if err != nil {
		panic("failed to parse global policy:" + err.Error())
	}

	return globalPolicy
}
