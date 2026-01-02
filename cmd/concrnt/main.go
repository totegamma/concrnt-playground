package main

import (
	"fmt"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel/trace"
	"log"
	"os"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/infra/config"
	"github.com/totegamma/concrnt-playground/internal/infra/database"
	"github.com/totegamma/concrnt-playground/internal/infra/gateway"
	"github.com/totegamma/concrnt-playground/internal/infra/repository"
	"github.com/totegamma/concrnt-playground/internal/present/rest"
	"github.com/totegamma/concrnt-playground/internal/present/rest/middleware"
	"github.com/totegamma/concrnt-playground/internal/service"
	"github.com/totegamma/concrnt-playground/internal/usecase"
	"github.com/totegamma/concrnt-playground/internal/utils"
)

var (
	version      = "unknown"
	buildMachine = "unknown"
	buildTime    = "unknown"
	goVersion    = "unknown"
)

func main() {

	fmt.Fprint(os.Stderr, concrnt.Banner)

	conf, err := config.Load("/etc/concrnt/config/config.yaml")
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	globalConfig := conf.GlobalConfig()

	log.Printf("Concrnt %s starting...", version)
	log.Printf("Config loaded! I am: %s @ %s on %s", globalConfig.CCID, globalConfig.FQDN, globalConfig.Layer)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.CORS())

	if conf.Server.EnableTrace {
		cleanup, err := utils.SetupTraceProvider(conf.Server.TraceEndpoint, conf.NodeInfo.FQDN+"/ccapi", version)
		if err != nil {
			panic(err)
		}
		defer cleanup()

		skipper := otelecho.WithSkipper(
			func(c echo.Context) bool {
				return c.Path() == "/metrics" || c.Path() == "/health"
			},
		)
		e.Use(otelecho.Middleware(conf.NodeInfo.FQDN, skipper))

		e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				span := trace.SpanFromContext(c.Request().Context())
				c.Response().Header().Set("trace-id", span.SpanContext().TraceID().String())
				return next(c)
			}
		})
	}

	softwareInfo := concrnt.SoftwareInfo{
		Version:      version,
		BuildMachine: buildMachine,
		BuildTime:    buildTime,
		GoVersion:    goVersion,
	}

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
	auth := service.NewAuthService(&globalConfig, cl)

	recordRepo := repository.NewRecordRepository(db, signal)
	recordUC := usecase.NewRecordUsecase(recordRepo)

	chunklineRepo := repository.NewChunklineRepository(db)
	chunklineGateway := gateway.NewChunklineGateway(cl)
	chunklineUC := usecase.NewChunklineUsecase(chunklineRepo, chunklineGateway)

	serverRepo := repository.NewServerRepository(&globalConfig, db, cl)
	serverUC := usecase.NewServerUsecase(serverRepo)

	entityRepo := repository.NewEntityRepository(db, cl, globalConfig)
	entityUC := usecase.NewEntityUsecase(entityRepo)

	authMiddleware := middleware.NewAuthMiddleware(auth, globalConfig)

	e.Use(authMiddleware.IdentifyIdentity)

	handler := rest.NewHandler(globalConfig, softwareInfo, recordUC, chunklineUC, serverUC, entityUC, signal)
	handler.RegisterRoutes(e)

	e.Logger.Fatal(e.Start(":8000"))

}
