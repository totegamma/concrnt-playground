package main

import (
	"encoding/json"

	"github.com/totegamma/concrnt-playground/policy"
)

var globalPolicyJson = `
{
	"statements": {
		"net.concrnt.core.commit.delete": [
			{
				"emit": "allow",
				"condition": {
					"op": "Eq",
					"args": [
						{
							"op": "Load",
							"const": "requester.ccid"
						},
						{
							"op": "Load",
							"const": "this.author"
						}
					]
				}
			}
		],
		"net.concrnt.core.resolve": [
			{
				"emit": "ok",
				"condition": {
					"op": "Const",
					"const": true
				}
			},
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
									"const": "requester.ccid"
								},
								{
									"op": "Load",
									"const": "this.author"
								}
							]
						},
						{
							"op": "Eq",
							"args": [
								{
									"op": "Load",
									"const": "requester.ccid"
								},
								{
									"op": "CCUriOwner",
									"args": [
										{
											"op": "Load",
											"const": "this.key"
										}
									]
								}
							]
						}
					]
				}
			}
		]
	}
}`

func GetGlobalPolicy() policy.Policy {
	globalPolicy := policy.Policy{}
	err := json.Unmarshal([]byte(globalPolicyJson), &globalPolicy)
	if err != nil {
		panic("failed to parse global policy:" + err.Error())
	}

	return globalPolicy
}
