package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/jwt"
)

var tracer = otel.Tracer("auth")

type AuthService struct {
	config *domain.Config
	client *client.Client
}

func NewAuthService(
	config *domain.Config,
	client *client.Client,
) *AuthService {
	return &AuthService{
		config: config,
		client: client,
	}
}

type AuthResult struct {
	CCID string
}

func (s *AuthService) AuthJwt(ctx context.Context, token string) (*AuthResult, error) {
	ctx, span := tracer.Start(ctx, "Auth.Service.AuthJwt")
	defer span.End()

	header, claims, err := jwt.Validate(token)
	if err != nil {
		span.RecordError(errors.Wrap(err, "jwt validation failed"))
		return nil, err
	}

	if claims.Audience != s.config.FQDN {
		err := fmt.Errorf("jwt audience mismatch: expected %s, got %s", s.config.FQDN, claims.Audience)
		span.RecordError(err)
		return nil, err
	}

	if claims.Subject != "concrnt" {
		err := fmt.Errorf("invalid subject")
		span.RecordError(err)
		return nil, err
	}

	keyID := header.KeyID
	if keyID == "" {
		keyID = claims.Issuer
	}

	var ccid string
	if concrnt.IsCCID(keyID) {
		ccid = keyID

		return &AuthResult{CCID: ccid}, nil
	} else if concrnt.IsCKID(keyID) {
		// TODO! not implemented
		err := fmt.Errorf("ckid not supported yet")
		span.RecordError(err)
		return nil, err
	} else {
		span.RecordError(fmt.Errorf("invalid issuer"))
		return nil, fmt.Errorf("invalid issuer")
	}
}
