package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	dsn := "host=localhost user=postgres password=postgres dbname=postgres port=5432 sslmode=disable"

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             300 * time.Millisecond, // Slow SQL threshold
			LogLevel:                  logger.Warn,            // Log level
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,                   // Enable color
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
		Logger:         gormLogger,
	})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(
		&CommitLog{},
		&CommitOwner{},
		&Record{},
		&RecordKey{},
		&CollectionMember{},
		&Association{},
	)

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	e.POST("/commit", func(c echo.Context) error {
		ctx := c.Request().Context()

		var sd SignedDocument
		err := c.Bind(&sd)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
		}

		err = HandleCommit(ctx, db, sd)
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
		split := strings.Split(strings.TrimPrefix(path, "/"), "/")
		key := split[0]

		if len(split) == 1 {
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
		} else {
			switch split[1] {
			case "childs":
				childRecords, err := handleGetChilds(ctx, db, fmt.Sprintf("cc://%s/%s", owner, split[0]))
				if err != nil {
					return c.JSON(http.StatusNotFound, echo.Map{"error": "resource not found", "details": err.Error()})
				}
				return c.JSON(200, echo.Map{
					"children": childRecords,
				})
			default:
				return c.JSON(http.StatusBadRequest, echo.Map{"error": "unsupported resource path"})
			}
		}
	})

	e.Logger.Fatal(e.Start(":8000"))

}
