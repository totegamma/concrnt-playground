package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/infra/config"
	"github.com/totegamma/concrnt-playground/internal/infra/database"
	"github.com/totegamma/concrnt-playground/internal/infra/gateway"
	"github.com/totegamma/concrnt-playground/internal/infra/repository"
	"github.com/totegamma/concrnt-playground/internal/present/rest"
	"github.com/totegamma/concrnt-playground/internal/service"
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

	mc := database.NewMemcached(conf.Server.MemcachedAddr)
	defer mc.Close()

	redis := database.NewRedis(conf.Server.RedisAddr, "", conf.Server.RedisDB)

	cl := client.New(conf.Server.GatewayAddr)
	signal := service.NewSignalService(redis)

	recordRepo := repository.NewRecordRepository(db, signal)
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
