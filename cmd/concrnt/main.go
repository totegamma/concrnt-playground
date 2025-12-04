package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/application"
	"github.com/totegamma/concrnt-playground/internal/config"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database/models"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
)

func main() {

	conf, err := config.Load("/etc/concrnt/config/config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	db, err := database.NewPostgres(conf.Server.PostgresDsn)
	if err != nil {
		panic("failed to connect database")
	}

	err = database.MigratePostgres(db)
	if err != nil {
		panic("failed to migrate database")
	}

	cl := client.New(conf.Server.GatewayAddr)

	recordRepo := repository.NewRecordRepository(db)
	recordApp := application.NewRecordApplication(recordRepo)

	chunklineRepo := repository.NewChunklineRepository(db)
	chunklineApp := application.NewChunklineApplication(chunklineRepo, cl)

	serverRepo := repository.NewServerRepository(db, cl)
	serverApp := application.NewServerApplication(serverRepo)

	entityRepo := repository.NewEntityRepository(db, cl)
	entityApp := application.NewEntityApplication(entityRepo)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/.well-known/concrnt", func(c echo.Context) error {
		wellknown := concrnt.WellKnownConcrnt{
			Version: "2.0",
			Domain:  conf.NodeInfo.FQDN,
			CSID:    conf.NodeInfo.CSID,
			Layer:   conf.NodeInfo.Layer,
			Endpoints: map[string]string{
				"net.concrnt.core.resource":         "/resource/{uri}",
				"net.concrnt.core.commit":           "/commit",
				"net.concrnt.world.register":        "/api/v1/register",
				"net.concrnt.world.timeline.recent": "/api/v1/timeline/recent",
			},
		}
		return c.JSON(http.StatusOK, wellknown)
	})

	e.POST("/commit", func(c echo.Context) error {
		ctx := c.Request().Context()

		var sd concrnt.SignedDocument
		err := c.Bind(&sd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

		err = recordApp.Commit(ctx, sd)

		return c.JSON(200, echo.Map{"status": "ok"})
	})

	e.GET("/resource/:uri", func(c echo.Context) error {
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

		owner, key, err := concrnt.ParseCCURI(uriString)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid uri"})
		}

		if key == "" {
			resolver := c.QueryParam("resolver")
			if concrnt.IsCCID(owner) {
				entity, err := entityApp.Get(ctx, owner, resolver)
				if err != nil {
					return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
				}
				return c.JSON(http.StatusOK, entity)
			}

			wkc, err := serverApp.Resolve(ctx, owner)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			return c.JSON(http.StatusOK, wkc)
		}

		accept := c.Request().Header.Get("Accept")

		if accept == "application/chunkline+json" {
			value, err := chunklineApp.GetChunklineManifest(ctx, uriString)
			if err != nil {
				if strings.Contains(err.Error(), "record not found") {
					return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
				}
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			return c.JSON(200, value)
		} else {
			value, err := recordApp.Get(ctx, uri.String())
			if err != nil {
				if strings.Contains(err.Error(), "record not found") {
					return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
				}
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
			}
			return c.JSON(200, value)
		}
	})

	e.GET("/chunkline/:owner/:id/:chunk/itr", func(c echo.Context) error {
		ctx := c.Request().Context()
		uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

		chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid chunk id"})
		}

		results, err := chunklineApp.LookupLocalItrs(ctx, []string{uri}, chunkID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}

		return c.String(200, strconv.FormatInt(results[uri], 10))
	})

	e.GET("/chunkline/:owner/:id/:chunk/body", func(c echo.Context) error {
		ctx := c.Request().Context()
		uri := concrnt.ComposeCCURI(c.Param("owner"), c.Param("id"))

		chunkID, err := strconv.ParseInt(c.Param("chunk"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid chunk id"})
		}
		results, err := chunklineApp.LoadLocalBody(ctx, uri, chunkID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(200, results)
	})

	e.GET("/api/v1/register", func(c echo.Context) error {
		ctx := c.Request().Context()
		var req concrnt.RegisterRequest[models.EntityMeta]
		err := c.Bind(&req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}
		err = entityApp.Register(ctx, req.AffiliationDocument, req.AffiliationSignature, req.Meta)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(200, echo.Map{"status": "ok"})
	})

	e.GET("/api/v1/timeline/recent", func(c echo.Context) error {
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
		results, err := chunklineApp.GetRecent(ctx, uris, until)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(200, results)
	})

	e.Logger.Fatal(e.Start(":8000"))

}
