package rest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/present/rest/presenter"
	"github.com/totegamma/concrnt-playground/internal/usecase"
	"github.com/totegamma/concrnt-playground/schemas"
)

type Handler struct {
	config    domain.Config
	record    *usecase.RecordUsecase
	chunkline *usecase.ChunklineUsecase
	server    *usecase.ServerUsecase
	entity    *usecase.EntityUsecase
}

func NewHandler(
	config domain.Config,
	record *usecase.RecordUsecase,
	chunkline *usecase.ChunklineUsecase,
	server *usecase.ServerUsecase,
	entity *usecase.EntityUsecase,
) *Handler {
	return &Handler{
		config:    config,
		record:    record,
		chunkline: chunkline,
		server:    server,
		entity:    entity,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/.well-known/concrnt", h.handleWellKnown)
	e.POST("/commit", h.handleCommit)
	e.GET("/resource/:uri", h.handleResource)
	e.GET("/chunkline/:owner/:id/:chunk/itr", h.handleChunklineItr)
	e.GET("/chunkline/:owner/:id/:chunk/body", h.handleChunklineBody)
	e.POST("/api/v1/register", h.handleRegister)
	e.GET("/api/v1/timeline/recent", h.handleTimelineRecent)
}

func (h *Handler) handleWellKnown(c echo.Context) error {
	wellknown := concrnt.WellKnownConcrnt{
		Version: "2.0",
		Domain:  h.config.FQDN,
		CSID:    h.config.CSID,
		Layer:   h.config.Layer,
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
		return presenter.BadRequest(c, err)
	}

	var doc concrnt.Document[any]
	if err := json.Unmarshal([]byte(sd.Document), &doc); err != nil {
		return presenter.BadRequest(c, err)
	}

	var deleteURI *string
	if doc.Schema != nil && *doc.Schema == schemas.DeleteURL {
		var deleteDoc concrnt.Document[schemas.Delete]
		if err := json.Unmarshal([]byte(sd.Document), &deleteDoc); err != nil {
			return presenter.BadRequest(c, err)
		}
		uri := string(deleteDoc.Value)
		deleteURI = &uri
	}

	err = h.record.Commit(ctx, usecase.CommitInput{
		Document: doc,
		Raw:      sd,
		Delete:   deleteURI,
	})
	if err != nil {
		return presenter.InternalError(c, err)
	}

	return presenter.OK(c, echo.Map{"status": "ok"})
}

func (h *Handler) handleResource(c echo.Context) error {
	ctx := c.Request().Context()

	escaped := c.Param("uri")
	uriString, err := url.QueryUnescape(escaped)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid uri")
	}
	uri, err := url.Parse(uriString)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid uri")
	}

	if uri.Scheme == "http" || uri.Scheme == "https" {
		return c.JSON(http.StatusSeeOther, echo.Map{"location": uri.String()})
	}

	if uri.Scheme != "cc" {
		return presenter.BadRequestMessage(c, "unsupported uri scheme")
	}

	hint := c.QueryParam("hint")

	owner, key, err := concrnt.ParseCCURI(uriString)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid uri")
	}

	if key == "" {
		if concrnt.IsCCID(owner) {
			entity, err := h.entity.Get(ctx, owner, hint)
			if err != nil {
				return presenter.InternalError(c, err)
			}
			return presenter.OK(c, entity)
		}

		wkc, err := h.server.Resolve(ctx, owner, hint)
		if err != nil {
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, wkc.WellKnown)
	}

	accept := c.Request().Header.Get("Accept")

	if accept == "application/chunkline+json" {
		value, err := h.chunkline.GetChunklineManifest(ctx, uriString)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return presenter.NotFound(c, "resource not found")
			}
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, value)
	}

	value, err := h.record.Get(ctx, uri.String())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return presenter.NotFound(c, "resource not found")
		}
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, value)
}

func (h *Handler) handleChunklineItr(c echo.Context) error {
	ctx := c.Request().Context()
	uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

	chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid chunk id")
	}

	results, err := h.chunkline.LookupLocalItrs(ctx, []string{uri}, chunkID)
	if err != nil {
		return presenter.InternalError(c, err)
	}

	return c.String(http.StatusOK, strconv.FormatInt(results[uri], 10))
}

func (h *Handler) handleChunklineBody(c echo.Context) error {
	ctx := c.Request().Context()
	uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

	chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid chunk id")
	}
	results, err := h.chunkline.LoadLocalBody(ctx, uri, chunkID)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, results)
}

func (h *Handler) handleRegister(c echo.Context) error {
	ctx := c.Request().Context()
	var req concrnt.RegisterRequest[domain.EntityMeta]
	err := c.Bind(&req)
	if err != nil {
		return presenter.BadRequest(c, err)
	}

	var doc concrnt.Document[schemas.Affiliation]
	if err := json.Unmarshal([]byte(req.AffiliationDocument), &doc); err != nil {
		return presenter.BadRequest(c, err)
	}

	err = h.entity.Register(ctx, usecase.EntityRegisterInput{
		Document:  doc,
		Raw:       req.AffiliationDocument,
		Signature: req.AffiliationSignature,
		Meta:      req.Meta,
	})
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok"})
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
			return presenter.BadRequestMessage(c, "invalid until parameter")
		}
		until = time.Unix(untilInt, 0).UTC()
	}
	limit := 16
	limitStr := c.QueryParam("limit")
	if limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err != nil {
			return presenter.BadRequestMessage(c, "invalid limit parameter")
		}
		limit = limitInt
	}
	if limit > 64 {
		limit = 64
	}

	results, err := h.chunkline.GetRecent(ctx, uris, until, limit)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, results)
}
