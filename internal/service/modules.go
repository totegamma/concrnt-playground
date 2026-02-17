package service

import (
	"encoding/json"
	"maps"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
)

type ModuleManager struct {
	baseEndpoints map[string]concrnt.ConcrntEndpoint
	services      []domain.Service
	endpoints     map[string]concrnt.ConcrntEndpoint
}

func NewModuleManager(
	baseEndpoints map[string]concrnt.ConcrntEndpoint,
	services []domain.Service,
) *ModuleManager {

	manager := ModuleManager{
		baseEndpoints: baseEndpoints,
		services:      services,
		endpoints:     make(map[string]concrnt.ConcrntEndpoint),
	}

	go func() {
		manager.UpdateEndpointRoutine()
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			manager.UpdateEndpointRoutine()
		}
	}()

	return &manager
}

func (m *ModuleManager) UpdateEndpointRoutine() {

	endpoints := make(map[string]concrnt.ConcrntEndpoint)

	maps.Copy(endpoints, m.baseEndpoints)

	client := &http.Client{}

	for _, service := range m.services {
		var resp *http.Response
		var err error

		url := "http://" + service.Host + ":" + strconv.Itoa(service.Port) + "/cc-info"
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}

		resp, err = client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var info domain.CCInfo
		err = json.NewDecoder(resp.Body).Decode(&info)
		if err != nil {
			continue
		}

		for key, endpoint := range info.Endpoints {
			endpoint.Template = path.Join(service.Path, endpoint.Template)
			endpoints[key] = endpoint
		}
	}

	m.endpoints = endpoints
}

func (m *ModuleManager) GetEndpoints() map[string]concrnt.ConcrntEndpoint {
	return m.endpoints
}
