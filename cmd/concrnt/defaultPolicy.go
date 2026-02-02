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
				"commit.delete": [
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
