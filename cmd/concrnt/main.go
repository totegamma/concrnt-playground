package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/bradfitz/gomemcache/memcache"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/infra/config"
	"github.com/totegamma/concrnt-playground/internal/infra/database"
	"github.com/totegamma/concrnt-playground/internal/infra/gateway"
	"github.com/totegamma/concrnt-playground/internal/infra/repository"
	"github.com/totegamma/concrnt-playground/internal/present/rest"
	"github.com/totegamma/concrnt-playground/internal/usecase"
)

func main() {

	conf, err := config.Load("/etc/concrnt/config/config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	globalConfig := conf.GlobalConfig()

	db, err := database.NewPostgres(conf.Server.PostgresDsn)
	if err != nil {
		panic("failed to connect database")
	}

	err = database.MigratePostgres(db)
	if err != nil {
		panic("failed to migrate database")
	}

	mc := memcache.New(conf.Server.MemcachedAddr)
	defer mc.Close()

	cl := client.New(conf.Server.GatewayAddr)

	recordRepo := repository.NewRecordRepository(db)
	recordUC := usecase.NewRecordUsecase(recordRepo)

	chunklineRepo := repository.NewChunklineRepository(db)
	chunklineGateway := gateway.NewChunklineGateway(cl)
	chunklineUC := usecase.NewChunklineUsecase(chunklineRepo, chunklineGateway)

	serverRepo := repository.NewServerRepository(db, cl)
	serverUC := usecase.NewServerUsecase(serverRepo)

	entityRepo := repository.NewEntityRepository(db, cl, globalConfig)
	entityUC := usecase.NewEntityUsecase(entityRepo)

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	handler := rest.NewHandler(globalConfig, recordUC, chunklineUC, serverUC, entityUC)
	handler.RegisterRoutes(e)

	e.Logger.Fatal(e.Start(":8000"))

}
