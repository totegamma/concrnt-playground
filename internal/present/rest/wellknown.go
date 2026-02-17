package rest

import (
	"encoding/json"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/present/rest/presenter"
)

type WellKnownHandler struct {
	config   *domain.Config
	info     concrnt.SoftwareInfo
	services []domain.Service

	endpoints map[string]concrnt.ConcrntEndpoint
}

func NewWellKnownHandler(
	config *domain.Config,
	info concrnt.SoftwareInfo,
	services []domain.Service,
) *WellKnownHandler {

	handler := WellKnownHandler{
		config:    config,
		info:      info,
		services:  services,
		endpoints: basicEndpoints,
	}

	go func() {
		handler.UpdateEndpointRoutine()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				handler.UpdateEndpointRoutine()
			}
		}
	}()

	return &handler
}

var basicEndpoints = map[string]concrnt.ConcrntEndpoint{
	"net.concrnt.core.resolve": {
		Template: "/resolve",
		Method:   "GET",
		Query:    &[]string{"uri"},
	},
	"net.concrnt.core.commit": {
		Template: "/commit",
		Method:   "POST",
	},
	"net.concrnt.core.query": {
		Template: "/query",
		Method:   "GET",
		Query:    &[]string{"prefix", "schema", "since", "until", "limit", "order"},
	},
	"net.concrnt.core.associations": {
		Template: "/associations",
		Method:   "GET",
		Query:    &[]string{"uri", "schema", "variant", "author"},
	},
	"net.concrnt.core.association-counts": {
		Template: "/association-counts",
		Method:   "GET",
		Query:    &[]string{"uri", "schema"},
	},
	"net.concrnt.core.realtime": {
		Template: "/realtime",
		Method:   "GET",
	},
	"net.concrnt.world.register": {
		Template: "/api/v1/register",
		Method:   "POST",
	},
	"net.concrnt.world.timeline.recent": {
		Template: "/api/v1/timeline/recent",
		Method:   "GET",
		Query:    &[]string{"uris", "until", "limit"},
	},
	"net.concrnt.core.known-servers": {
		Template: "/known-servers",
		Method:   "GET",
	},
}

func (p *WellKnownHandler) UpdateEndpointRoutine() {

	endpoints := make(map[string]concrnt.ConcrntEndpoint)

	for key, endpoint := range basicEndpoints {
		endpoints[key] = endpoint
	}

	client := &http.Client{}

	for _, service := range p.services {
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

	p.endpoints = endpoints

}

func (p *WellKnownHandler) RegisterRoutes(e *echo.Echo) {
	e.GET("/.well-known/concrnt", p.handleWellKnown)
}

func (p *WellKnownHandler) handleWellKnown(c echo.Context) error {
	wellknown := concrnt.WellKnownConcrnt{
		Version:      "2.0",
		Domain:       p.config.FQDN,
		CSID:         p.config.CSID,
		Layer:        p.config.Layer,
		Dimension:    p.config.Dimension,
		Endpoints:    p.endpoints,
		SoftwareInfo: p.info,
		Meta:         p.config.Meta,
	}
	return presenter.OK(c, wellknown)
}
