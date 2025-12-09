package main

import (
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/config"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/database"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/gateway"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
	"github.com/totegamma/concrnt-playground/internal/interface/rest"
	"github.com/totegamma/concrnt-playground/internal/usecase"
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

	entityRepo := repository.NewEntityRepository(db, cl)
	entityUC := usecase.NewEntityUsecase(entityRepo)

	e := echo.New()
	// e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	handler := rest.NewHandler(conf.NodeInfo, recordUC, chunklineUC, serverUC, entityUC)
	handler.RegisterRoutes(e)

	e.Logger.Fatal(e.Start(":8000"))

}
