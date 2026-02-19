package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/impl/interop"
	"github.com/totegamma/concrnt-playground/impl/tags"
)

type Proxy struct {
	services []interop.Service
}

func NewProxy(services []interop.Service) *Proxy {
	return &Proxy{services: services}
}

func (p *Proxy) RegisterRoutes(e *echo.Echo) {

	cors := echomiddleware.CORS()

	for _, service := range p.services {

		targetUrl, err := url.Parse("http://" + service.Host + ":" + strconv.Itoa(service.Port))
		if err != nil {
			panic(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(targetUrl)

		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = targetUrl.Scheme
			req.URL.Host = targetUrl.Host
			req.URL.Path = path.Join(targetUrl.Path, req.URL.Path)

			otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
		}

		proxy.Transport = otelhttp.NewTransport(http.DefaultTransport)

		middlewares := []echo.MiddlewareFunc{
			// authService.RateLimiter(service.RateLimitConf),
		}
		if service.InjectCors {
			middlewares = append(middlewares, cors)
		}

		handler := func(c echo.Context) error {
			ctx := c.Request().Context()
			c.Response().Header().Set("cc-service", service.Name)

			requester, ok := ctx.Value(interop.RequesterCtxKey).(concrnt.Entity)
			if ok {
				serialized, err := json.Marshal(requester)
				if err != nil {
					return err
				}
				c.Request().Header.Set(interop.RequesterHeader, string(serialized))
			}

			tag, ok := ctx.Value(interop.RequesterTagCtxKey).(tags.Tags)
			if ok {
				tagStr := tag.ToString()
				c.Request().Header.Set(interop.RequesterTagHeader, tagStr)
			}

			proxy.ServeHTTP(c.Response(), c.Request())
			return nil
		}

		e.Any(service.Path, handler, middlewares...)
		e.Any(service.Path+"/*", handler, middlewares...)
	}
}
