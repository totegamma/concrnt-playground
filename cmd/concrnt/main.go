package main

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/internal/application"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
)

func main() {
	dsn := "host=db user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"

	db, err := database.NewPostgres(dsn)
	if err != nil {
		panic("failed to connect database")
	}

	err = database.MigratePostgres(db)
	if err != nil {
		panic("failed to migrate database")
	}

	recordRepo := repository.NewRecordRepository(db)
	recordApp := application.NewRecordApplication(recordRepo)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.GET("/.well-known/concrnt", func(c echo.Context) error {
		wellknown := concrnt.WellKnownConcrnt{
			Version: "2.0",
			Domain:  "",
			CSID:    "",
			Layer:   "",
			Endpoints: map[string]string{
				"net.concrnt.core.entity":   "/entity/{ccid}",
				"net.concrnt.core.resource": "/resource/{uri}",
				"net.concrnt.core.commit":   "/commit",
				// "net.concrnt.world.register":        "/api/v1/register",
				// "net.concrnt.world.chunkline.query": "/api/v1/timeline",
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

		value, err := recordApp.Get(ctx, uri.String())
		if err != nil {
			if strings.Contains(err.Error(), "record not found") {
				return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
			}
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
		}
		return c.JSON(200, value)

	})

	e.Logger.Fatal(e.Start(":8000"))

}
