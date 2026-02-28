package main

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel/trace"
	"log"
	"log/slog"
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
	"github.com/totegamma/concrnt-playground/internal/worker"
)

var (
	version      = "unknown"
	buildMachine = "unknown"
	buildTime    = "unknown"
	goVersion    = "unknown"
)

type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {

	r.AddAttrs(slog.String("type", "app"))

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(slog.String("traceID", span.SpanContext().TraceID().String()))
		r.AddAttrs(slog.String("spanID", span.SpanContext().SpanID().String()))
	}

	return h.Handler.Handle(ctx, r)
}

func main() {

	lh := &CustomHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})}
	slogger := slog.New(lh)
	slog.SetDefault(slogger)

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

	e.Use(echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
		Skipper: func(c echo.Context) bool {
			return c.Path() == "/metrics" || c.Path() == "/health" || c.Path() == "/.well-known/concrnt"
		},
	}))

	e.Use(echomiddleware.Recover())

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
	policy := service.NewPolicyService(GetGlobalPolicy())

	serverRepo := repository.NewServerRepository(&globalConfig, db, cl)
	serverUC := usecase.NewServerUsecase(serverRepo)

	entityRepo := repository.NewEntityRepository(db, cl, globalConfig)
	entityUC := usecase.NewEntityUsecase(entityRepo)

	recordRepo := repository.NewRecordRepository(db)
	recordUC := usecase.NewRecordUsecase(recordRepo, &globalConfig, cl, entityUC, signal, policy)

	chunklineRepo := repository.NewChunklineRepository(db)
	chunklineGateway := gateway.NewChunklineGateway(cl)
	chunklineUC := usecase.NewChunklineUsecase(chunklineRepo, chunklineGateway)

	subscriber := worker.NewSubscriber(&globalConfig, cl, signal)
	subscriber.Start(context.Background())

	authMiddleware := middleware.NewAuthMiddleware(auth, globalConfig, entityRepo)

	e.Use(authMiddleware.IdentifyIdentity)

	moduleManager := service.NewModuleManager(basicEndpoints, conf.Services)

	wellKnownHandler := rest.NewWellKnownHandler(&globalConfig, softwareInfo, moduleManager)
	wellKnownHandler.RegisterRoutes(e)

	apiHandler := rest.NewHandler(globalConfig, recordUC, chunklineUC, serverUC, entityUC, signal, moduleManager)
	apiHandler.RegisterRoutes(e)

	proxy := rest.NewProxy(conf.Services)
	proxy.RegisterRoutes(e)

	e.Logger.Fatal(e.Start(":8000"))

}

var basicEndpoints = map[string]string{
	"net.concrnt.core.resolve":            "/resolve?uri={uri}",
	"net.concrnt.core.commit":             "/commit",
	"net.concrnt.core.query":              "/query{?prefix,schema,since,until,limit,order}",
	"net.concrnt.core.associations":       "/associations{?uri,schema,variant,author}",
	"net.concrnt.core.association-counts": "/association-counts{?uri,schema}",
	"net.concrnt.core.realtime":           "/realtime",
	"net.concrnt.world.register":          "/api/v1/register",
	"net.concrnt.world.timeline.recent":   "/api/v1/timeline/recent{?uris,until,limit}",
	"net.concrnt.core.known-servers":      "/known-servers",
}
