package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/present/rest/presenter"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/usecase"
	"github.com/totegamma/concrnt-playground/schemas"
)

type Handler struct {
	config    domain.Config
	info      concrnt.SoftwareInfo
	record    *usecase.RecordUsecase
	chunkline *usecase.ChunklineUsecase
	server    *usecase.ServerUsecase
	entity    *usecase.EntityUsecase
	signal    *service.SignalService
}

func NewHandler(
	config domain.Config,
	info concrnt.SoftwareInfo,
	record *usecase.RecordUsecase,
	chunkline *usecase.ChunklineUsecase,
	server *usecase.ServerUsecase,
	entity *usecase.EntityUsecase,
	signal *service.SignalService,
) *Handler {
	return &Handler{
		config:    config,
		info:      info,
		record:    record,
		chunkline: chunkline,
		server:    server,
		entity:    entity,
		signal:    signal,
	}
}

func (h *Handler) RegisterRoutes(e *echo.Echo) {
	e.GET("/.well-known/concrnt", h.handleWellKnown)
	e.POST("/commit", h.handleCommit)
	e.GET("/resolve", h.handleResource)
	e.GET("/query", h.handleQuery)
	e.GET("/chunkline/:owner/:key/:chunk/itr", h.handleChunklineItr)
	e.GET("/chunkline/:owner/:key/:chunk/body", h.handleChunklineBody)
	e.POST("/api/v1/register", h.handleRegister)
	e.GET("/api/v1/timeline/recent", h.handleTimelineRecent)
	e.GET("/associations", h.handleAssociations)
	e.GET("/association-counts", h.handleAssociationCounts)
	e.GET("/known-servers", h.handleKnownServers)
	e.GET("/realtime", h.handleRealtime)

	e.GET("/internal/signal/subscriptions", h.handleCurrentSubs)

	e.GET("/health", func(c echo.Context) (err error) {
		// ctx := c.Request().Context()

		/*
			err = sqlDB.Ping()
			if err != nil {
				return c.String(http.StatusInternalServerError, "db error")
			}

			err = rdb.Ping(ctx).Err()
			if err != nil {
				return c.String(http.StatusInternalServerError, "redis error")
			}
		*/

		return c.String(http.StatusOK, "ok")
	})

	// Legacy endpoints
	e.GET("/api/v1/domain", h.handleLegacyDomain)
	e.GET("/api/v1/entity/:id", h.handleLegacyEntity)
	e.GET("/api/v1/entities", h.handleLegacyEntities)
	e.GET("/api/v1/timelines", h.handleLegacyTimelines)
	e.GET("/api/v1/profile/:owner/:semanticid", h.handleLegacyProfile)
	e.GET("/api/v1/profiles", h.handleLegacyProfiles)
}

func (h *Handler) handleWellKnown(c echo.Context) error {
	wellknown := concrnt.WellKnownConcrnt{
		Version:   "2.0",
		Domain:    h.config.FQDN,
		CSID:      h.config.CSID,
		Layer:     h.config.Layer,
		Dimension: h.config.Dimension,
		Endpoints: map[string]concrnt.ConcrntEndpoint{
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
		},
		SoftwareInfo: h.info,
		Meta:         h.config.Meta,
	}
	return presenter.OK(c, wellknown)
}

func (h *Handler) handleCommit(c echo.Context) error {
	ctx := c.Request().Context()

	var sd concrnt.SignedDocument
	err := c.Bind(&sd)
	if err != nil {
		return presenter.BadRequest(c, err)
	}

	err = h.record.Commit(ctx, sd)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionDenied) {
			return presenter.Forbidden(c, "permission denied")
		}
		return presenter.InternalError(c, err)
	}

	return presenter.OK(c, echo.Map{"status": "ok"})
}

func (h *Handler) handleResource(c echo.Context) error {
	ctx, span := tracer.Start(c.Request().Context(), "Handler.handleResource")
	defer span.End()

	uriEscaped := c.QueryParam("uri")
	uriString, err := url.PathUnescape(uriEscaped)
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

	parsed, err := concrnt.ParseCCURI(uriString)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid uri")
	}

	if parsed.Key == "" && parsed.CDID == "" {
		if concrnt.IsCCID(parsed.Owner) {
			entity, err := h.entity.Get(ctx, parsed.Owner, parsed.Hint)
			if err != nil {
				if errors.Is(err, domain.ErrPermissionDenied) {
					return presenter.Forbidden(c, "permission denied") // TODO: should be return NotFound
				}
			}
			return presenter.OK(c, entity)
		}

		if parsed.Owner == h.config.CSID {
			return c.Redirect(http.StatusFound, "https://"+h.config.FQDN+"/.well-known/concrnt")
		}

		wkc, err := h.server.Resolve(ctx, parsed.Owner, parsed.Hint)
		if err != nil {
			if errors.Is(err, domain.ErrPermissionDenied) {
				return presenter.Forbidden(c, "permission denied") // TODO: should be return NotFound
			}
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, wkc)
	}

	accept := c.Request().Header.Get("Accept")

	switch accept {
	case "application/chunkline+json":
		value, err := h.chunkline.GetChunklineManifest(ctx, uriString)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return presenter.NotFound(c, "resource not found")
			}
			if errors.Is(err, domain.ErrPermissionDenied) {
				return presenter.Forbidden(c, "permission denied") // TODO: should be return NotFound
			}
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, value)
	default:
		value, err := h.record.GetSigned(ctx, uri.String())
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return presenter.NotFound(c, "resource not found")
			}
			if errors.Is(err, domain.ErrPermissionDenied) {
				return presenter.Forbidden(c, "permission denied") // TODO: should be return NotFound
			}
			return presenter.InternalError(c, err)
		}

		var doc concrnt.Document[schemas.Reference]
		err = json.Unmarshal([]byte(value.Document), &doc)
		if err != nil {
			return presenter.OK(c, value)
		}

		if doc.Schema == schemas.ReferenceURL {
			c.Response().Header().Set("Location", "/resolve?uri="+url.PathEscape(doc.Value.Href))
			return c.JSON(http.StatusFound, value)
		}

		return presenter.OK(c, value)
	}

}

func (h *Handler) handleQuery(c echo.Context) error {
	ctx := c.Request().Context()

	prefix := c.QueryParam("prefix")
	if prefix == "" {
		return presenter.BadRequestMessage(c, "prefix parameter is required")
	}

	schema := c.QueryParam("schema")

	var since *time.Time
	sinceStr := c.QueryParam("since")
	if sinceStr != "" {
		parsed, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return presenter.BadRequestMessage(c, "invalid since parameter")
		}
		since = &parsed
	}

	var until *time.Time
	untilStr := c.QueryParam("until")
	if untilStr != "" {
		parsed, err := time.Parse(time.RFC3339, untilStr)
		if err != nil {
			return presenter.BadRequestMessage(c, "invalid until parameter")
		}
		until = &parsed
	}

	limit := 10
	limitStr := c.QueryParam("limit")
	if limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err != nil {
			return presenter.BadRequestMessage(c, "invalid limit parameter")
		}
		limit = limitInt
	}
	if limit > 100 {
		limit = 100
	}

	order := c.QueryParam("order")
	if order == "" {
		order = "desc"
	} else if order != "asc" && order != "desc" {
		return presenter.BadRequestMessage(c, "invalid order parameter")
	}

	results, err := h.record.Query(ctx, prefix, schema, since, until, limit, order)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, results)
}

func (h *Handler) handleChunklineItr(c echo.Context) error {
	ctx := c.Request().Context()
	key, err := url.PathUnescape(c.Param("key"))
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid key")
	}
	uri := concrnt.ComposeCCURI("cckv", c.Param("owner"), key)

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
	key, err := url.PathUnescape(c.Param("key"))
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid key")
	}
	uri := concrnt.ComposeCCURI("cckv", c.Param("owner"), key)

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

	err = h.entity.Register(ctx, req)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok"})
}

func (h *Handler) handleTimelineRecent(c echo.Context) error {
	ctx := c.Request().Context()
	uriString := c.QueryParam("uris")
	if uriString == "" {
		return presenter.OK(c, []any{})
	}
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

func (h *Handler) handleAssociations(c echo.Context) error {
	ctx := c.Request().Context()

	uri := c.QueryParam("uri")
	schema := c.QueryParam("schema")
	variant := c.QueryParam("variant")
	author := c.QueryParam("author")

	if uri == "" {
		return presenter.BadRequestMessage(c, "uri parameter is required")
	}

	records, err := h.record.GetAssociatedRecords(ctx, uri, schema, variant, author)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, records)

}

func (h *Handler) handleAssociationCounts(c echo.Context) error {
	ctx := c.Request().Context()

	uri := c.QueryParam("uri")
	schema := c.QueryParam("schema")

	if uri == "" {
		return presenter.BadRequestMessage(c, "uri parameter is required")
	}

	if schema == "" {
		counts, err := h.record.GetAssociatedRecordCountsBySchema(ctx, uri)
		if err != nil {
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, counts)
	} else {
		counts, err := h.record.GetAssociatedRecordCountsByVariant(ctx, uri, schema)
		if err != nil {
			return presenter.InternalError(c, err)
		}
		return presenter.OK(c, counts)
	}

}

func (h *Handler) handleCurrentSubs(c echo.Context) error {
	subs := h.signal.GetCurrentSubscriptions()
	return presenter.OK(c, subs)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *Handler) handleRealtime(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		slog.Error(
			"Failed to upgrade WebSocket",
			slog.String("error", err.Error()),
			slog.String("module", "socket"),
		)
		return err
	}
	defer func() {
		ws.Close()
	}()

	ctx := c.Request().Context()

	input := make(chan []string)
	defer close(input)
	output := make(chan concrnt.Event)
	defer close(output)

	go h.signal.Realtime(ctx, input, output)

	quit := make(chan struct{})

	go func() {
		for {
			var req concrnt.RealtimeRequest
			err := ws.ReadJSON(&req)
			if err != nil {

				wsErr, ok := err.(*websocket.CloseError)
				if ok {
					if !(wsErr.Code == websocket.CloseNormalClosure || wsErr.Code == websocket.CloseGoingAway) {
						slog.DebugContext(
							ctx, "WebSocket closed",
							slog.String("error", wsErr.Error()),
							slog.String("module", "socket"),
						)
					}
				} else {
					slog.ErrorContext(
						ctx, "Error reading message",
						slog.String("error", err.Error()),
						slog.String("module", "socket"),
					)
				}

				quit <- struct{}{}
				break
			}

			switch req.Type {
			case "listen":
				input <- req.Prefixes
				slog.DebugContext(
					ctx, fmt.Sprintf("Socket subscribe: %s", req.Prefixes),
					slog.String("module", "socket"),
				)
			case "h": // heartbeat
				// do nothing
			default:
				slog.InfoContext(
					ctx, "Unknown request type",
					slog.String("type", req.Type),
					slog.String("module", "socket"),
				)
			}
		}
	}()

	for {
		select {
		case <-quit:
			return nil
		case items := <-output:
			err := ws.WriteJSON(items)
			if err != nil {
				slog.ErrorContext(
					ctx, "Error writing message",
					slog.String("error", err.Error()),
					slog.String("module", "socket"),
				)
				return nil
			}
		}
	}
}

func (h *Handler) handleLegacyDomain(c echo.Context) error {
	domain := echo.Map{
		"fqdn":      h.config.FQDN,
		"csid":      h.config.CSID,
		"dimension": h.config.Dimension,
		"meta":      h.config.Meta,
	}
	return presenter.OK(c, echo.Map{"status": "ok", "content": domain})
}

func (h *Handler) handleLegacyEntity(c echo.Context) error {
	id := c.Param("id")
	ctx := c.Request().Context()

	entity, err := h.entity.Get(ctx, id, nil)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok", "content": entity})

}

func (h *Handler) handleLegacyEntities(c echo.Context) error {
	ctx := c.Request().Context()

	entities, err := h.entity.List(ctx)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok", "content": entities})
}

func (h *Handler) handleLegacyTimelines(c echo.Context) error {
	ctx := c.Request().Context()
	schemaEscaped := c.QueryParam("schema")
	schema, err := url.PathUnescape(schemaEscaped)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid schema")
	}

	results, err := h.record.Query(ctx, "", schema, nil, nil, 0, "desc")
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok", "content": results})
}

func (h *Handler) handleLegacyProfile(c echo.Context) error {
	owner, err := url.PathUnescape(c.Param("owner"))
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid owner")
	}
	semanticID, err := url.PathUnescape(c.Param("semanticid"))
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid semanticid")
	}

	if semanticID == "world.concrnt.p" {
		semanticID = "concrnt.world/main/profile"
	}

	uri := concrnt.ComposeCCURI("cckv", owner, semanticID)

	ctx := c.Request().Context()
	sd, err := h.record.GetSigned(ctx, uri)
	if err != nil {
		return presenter.InternalError(c, err)
	}

	doc, err := concrnt.ToLegacyDocument(sd)
	if err != nil {
		return presenter.InternalError(c, err)
	}

	return presenter.OK(c, echo.Map{"status": "ok", "content": doc})
}

func (h *Handler) handleLegacyProfiles(c echo.Context) error {
	ctx := c.Request().Context()

	schemaEscaped := c.QueryParam("schema")
	schema, err := url.PathUnescape(schemaEscaped)
	if err != nil {
		return presenter.BadRequestMessage(c, "invalid schema")
	}

	results, err := h.record.Query(ctx, "", schema, nil, nil, 0, "desc")
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, echo.Map{"status": "ok", "content": results})
}

func (h *Handler) handleKnownServers(c echo.Context) error {
	ctx := c.Request().Context()

	servers, err := h.server.List(ctx)
	if err != nil {
		return presenter.InternalError(c, err)
	}
	return presenter.OK(c, servers)
}
