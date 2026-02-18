package interop

import (
	"github.com/totegamma/concrnt-playground"
)

type Service struct {
	Name         string `yaml:"name"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Path         string `yaml:"path"`
	PreservePath bool   `yaml:"preservePath"`
	InjectCors   bool   `yaml:"injectCors"`
	Gone         bool   `yaml:"gone"`
}

type CCInfo struct {
	Name      string                             `json:"name"`
	Version   string                             `json:"version"`
	Endpoints map[string]concrnt.ConcrntEndpoint `json:"endpoints"`
}
