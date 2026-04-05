package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/impl/interop"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/internal/infra/repository"
	"github.com/totegamma/concrnt-playground/internal/service"
)

var tracer = otel.Tracer("auth")

type AuthMiddleware struct {
	auth   *service.AuthService
	config domain.Config
	server *repository.ServerRepository
	entity *repository.EntityRepository
}

func NewAuthMiddleware(
	auth *service.AuthService,
	config domain.Config,
	server *repository.ServerRepository,
	entity *repository.EntityRepository,
) *AuthMiddleware {
	return &AuthMiddleware{
		auth:   auth,
		config: config,
		server: server,
		entity: entity,
	}
}

func (s *AuthMiddleware) IdentifyIdentity(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx, span := tracer.Start(c.Request().Context(), "Auth.Service.IdentifyIdentity")
		defer span.End()

		referrer := c.Request().Header.Get(interop.ReferrerHeader)
		if referrer != "" {
			span.SetAttributes(attribute.String("Referrer", referrer))
			ctx = context.WithValue(ctx, interop.ReferrerCtxKey, referrer)
		}

		// # authtoken
		// 実体はjwtトークン
		// requesterが本人であることを証明するのに使う。
		authHeader := c.Request().Header.Get("authorization")

		if authHeader != "" {
			split := strings.Split(authHeader, " ")
			if len(split) != 2 {
				span.RecordError(fmt.Errorf("invalid authentication header"))
				goto skipCheckAuthorization
			}

			authType, token := split[0], split[1]
			if authType != "Bearer" {
				span.RecordError(fmt.Errorf("only Bearer is acceptable"))
				goto skipCheckAuthorization
			}

			result, err := s.auth.AuthJwt(ctx, token)
			if err != nil {
				span.RecordError(errors.Wrap(err, "AuthMiddleware.IdentifyIdentity: s.auth.AuthJwt failed"))
				goto skipCheckAuthorization
			}

			ent, err := s.entity.Get(ctx, result.CCID, result.Hint)
			if err != nil {
				span.RecordError(errors.Wrap(err, "AuthMiddleware.IdentifyIdentity: s.entity.Get failed"))
				goto skipCheckAuthorization
			}
			entityTag := ent.Tag()

			if entityTag.Has("_blocked") {
				err := fmt.Errorf("entity is blocked")
				return echo.NewHTTPError(403, err.Error())
			}

			srv, err := s.server.GetAndCacheByFQDN(ctx, ent.Domain)
			if err != nil {
				span.RecordError(errors.Wrap(err, "AuthMiddleware.IdentifyIdentity: s.server.GetAndCacheByFQDN failed"))
				goto skipCheckAuthorization
			}
			srvTag := srv.Tag()

			if srvTag.Has("_blocked") {
				err := fmt.Errorf("server is blocked")
				return echo.NewHTTPError(403, err.Error())
			}

			requester := concrnt.Entity{
				CCID:                 ent.ID,
				Domain:               ent.Domain,
				AffiliationDocument:  ent.AffiliationDocument,
				AffiliationSignature: ent.AffiliationSignature,
			}

			ctx = context.WithValue(ctx, interop.RequesterCtxKey, requester)
			span.SetAttributes(attribute.String("RequesterId", result.CCID))

			ctx = context.WithValue(ctx, interop.RequesterTagCtxKey, entityTag)
			span.SetAttributes(attribute.String("RequesterTag", entityTag.ToString()))
		}

	skipCheckAuthorization:
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}
