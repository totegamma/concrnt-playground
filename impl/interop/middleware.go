package interop

import (
	"context"
	"encoding/json"
	
	"github.com/labstack/echo/v4"

	"github.com/totegamma/concrnt-playground"
)

func ReceiveGatewayAuthPropagation(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, span := tracer.Start(c.Request().Context(), "Auth.Service.ReceiveGatewayAuthPropagation")
		defer span.End()

		requesterHeader := c.Request().Header.Get(RequesterHeader)

		if requesterHeader != "" {
			var requester concrnt.Entity
			err := json.Unmarshal([]byte(requesterHeader), &requester)
			if err == nil {
				ctx = context.WithValue(ctx, RequesterCtxKey, requester)
			}
		}

		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}
