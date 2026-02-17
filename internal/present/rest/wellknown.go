package rest

import (
	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/present/rest/presenter"
	"github.com/totegamma/concrnt-playground/internal/service"
)

type WellKnownHandler struct {
	config *domain.Config
	info   concrnt.SoftwareInfo
	mm     *service.ModuleManager
}

func NewWellKnownHandler(
	config *domain.Config,
	info concrnt.SoftwareInfo,
	mm *service.ModuleManager,
) *WellKnownHandler {

	return &WellKnownHandler{
		config: config,
		info:   info,
		mm:     mm,
	}
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
		Endpoints:    p.mm.GetEndpoints(),
		SoftwareInfo: p.info,
		Meta:         p.config.Meta,
	}
	return presenter.OK(c, wellknown)
}
