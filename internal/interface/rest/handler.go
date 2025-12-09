package rest

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/config"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/usecase"
)

type Handler struct {
	record    *usecase.RecordUsecase
	chunkline *usecase.ChunklineUsecase
	server    *usecase.ServerUsecase
	entity    *usecase.EntityUsecase
	nodeInfo  config.NodeInfo
}

func NewHandler(
	nodeInfo config.NodeInfo,
	record *usecase.RecordUsecase,
	chunkline *usecase.ChunklineUsecase,
	server *usecase.ServerUsecase,
	entity *usecase.EntityUsecase,
) *Handler {
	return &Handler{
		record:    record,
		chunkline: chunkline,
		server:    server,
		entity:    entity,
		nodeInfo:  nodeInfo,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/.well-known/concrnt", h.handleWellKnown)
	e.POST("/commit", h.handleCommit)
	e.GET("/resource/:uri", h.handleResource)
	e.GET("/chunkline/:owner/:id/:chunk/itr", h.handleChunklineItr)
	e.GET("/chunkline/:owner/:id/:chunk/body", h.handleChunklineBody)
	e.GET("/api/v1/register", h.handleRegister)
	e.GET("/api/v1/timeline/recent", h.handleTimelineRecent)
}

func (h *Handler) handleWellKnown(c echo.Context) error {
	wellknown := concrnt.WellKnownConcrnt{
		Version: "2.0",
		Domain:  h.nodeInfo.FQDN,
		CSID:    h.nodeInfo.CSID,
		Layer:   h.nodeInfo.Layer,
		Endpoints: map[string]string{
			"net.concrnt.core.resource":         "/resource/{uri}",
			"net.concrnt.core.commit":           "/commit",
			"net.concrnt.world.register":        "/api/v1/register",
			"net.concrnt.world.timeline.recent": "/api/v1/timeline/recent",
		},
	}
	return c.JSON(http.StatusOK, wellknown)
}

func (h *Handler) handleCommit(c echo.Context) error {
	ctx := c.Request().Context()

	var sd concrnt.SignedDocument
	err := c.Bind(&sd)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	err = h.record.Commit(ctx, sd)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

func (h *Handler) handleResource(c echo.Context) error {
	ctx := c.Request().Context()

	escaped := c.Param("uri")
	uriString, err := url.QueryUnescape(escaped)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid uri"})
	}
	uri, err := url.Parse(uriString)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid uri"})
	}

	if uri.Scheme == "http" || uri.Scheme == "https" {
		return c.JSON(http.StatusSeeOther, echo.Map{"location": uri.String()})
	}

	if uri.Scheme != "cc" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "unsupported uri scheme"})
	}

	hint := c.QueryParam("hint")

	owner, key, err := concrnt.ParseCCURI(uriString)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid uri"})
	}

	if key == "" {
		if concrnt.IsCCID(owner) {
			entity, err := h.entity.Get(ctx, owner, hint)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, entity)
		}

		wkc, err := h.server.Resolve(ctx, owner, hint)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, wkc.WellKnown)
	}

	accept := c.Request().Header.Get("Accept")

	if accept == "application/chunkline+json" {
		value, err := h.chunkline.GetChunklineManifest(ctx, uriString)
		if err != nil {
			if strings.Contains(err.Error(), "record not found") {
				return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
			}
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, value)
	}

	value, err := h.record.Get(ctx, uri.String())
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, value)
}

func (h *Handler) handleChunklineItr(c echo.Context) error {
	ctx := c.Request().Context()
	uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

	chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid chunk id"})
	}

	results, err := h.chunkline.LookupLocalItrs(ctx, []string{uri}, chunkID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.String(http.StatusOK, strconv.FormatInt(results[uri], 10))
}

func (h *Handler) handleChunklineBody(c echo.Context) error {
	ctx := c.Request().Context()
	uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

	chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid chunk id"})
	}
	results, err := h.chunkline.LoadLocalBody(ctx, uri, chunkID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, results)
}

func (h *Handler) handleRegister(c echo.Context) error {
	ctx := c.Request().Context()
	var req concrnt.RegisterRequest[domain.EntityMeta]
	err := c.Bind(&req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	err = h.entity.Register(ctx, req.AffiliationDocument, req.AffiliationSignature, req.Meta)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
}

func (h *Handler) handleTimelineRecent(c echo.Context) error {
	ctx := c.Request().Context()
	uriString := c.QueryParam("uris")
	uris := strings.Split(uriString, ",")
	untilStr := c.QueryParam("until")
	var until time.Time
	if untilStr == "" {
		until = time.Now().UTC()
	} else {
		untilInt, err := strconv.ParseInt(untilStr, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid until parameter"})
		}
		until = time.Unix(untilInt, 0).UTC()
	}
	limit := 16
	limitStr := c.QueryParam("limit")
	if limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid limit parameter"})
		}
		limit = limitInt
	}
	if limit > 64 {
		limit = 64
	}

	results, err := h.chunkline.GetRecent(ctx, uris, until, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, results)
}
