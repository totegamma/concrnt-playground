package main

import (
	"fmt"
	"strings"
	"net/http"
	"net/url"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)


func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(
		&CommitLog{},
		&CommitOwner{},
		&Record{},
		&RecordKey{},
	)


	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.POST("/commit", func(c echo.Context) error {
		ctx := c.Request().Context()

		var request Commit
		err := c.Bind(&request)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

		err = HandleCommit(ctx, db, request)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

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

		owner := uri.Host
		path := uri.Path

		fmt.Println("Owner:", owner)
		fmt.Println("Path:", path)

		key := strings.TrimPrefix(path, "/")

		record, err := handleGetRecordByKey(ctx, db, owner, key)
		if err != nil {
			record, err = handleGetRecordByID(ctx, db, key)
		}
		if err != nil {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found"})
		}

		return c.JSON(200, echo.Map{
			"content": record.Value,
		})
	})

	e.Logger.Fatal(e.Start(":8000"))

}

