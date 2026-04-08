package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/domain"
	"github.com/totegamma/concrnt-playground/jwt"
	"github.com/totegamma/concrnt-playground/schemas"
)

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
	Hint *string
}

func (s *AuthService) AuthJwt(ctx context.Context, token string) (*AuthResult, error) {
	ctx, span := tracer.Start(ctx, "Auth.Service.AuthJwt")
	defer span.End()

	header, claims, err := jwt.Parse(token)
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

	parsed, err := concrnt.ParseCCURI(keyID)
	if err != nil {
		span.RecordError(errors.Wrap(err, "failed to parse issuer as CCURI"))
		return nil, err
	}
	hint := parsed.Hint
	ccid := parsed.Owner

	if parsed.Key == "" { // login as raw key

		err = jwt.Validate(token, ccid)
		if err != nil {
			span.RecordError(errors.Wrap(err, "jwt signature validation failed"))
			return nil, err
		}

		return &AuthResult{
			CCID: ccid,
			Hint: hint,
		}, nil

	} else { // login as subkey

		var subKeyDoc concrnt.Document[schemas.Subkey]
		// TODO 署名を確認するstrictオプションが必要
		// TODO キーのキャッシュが必須
		err := s.client.GetRecord(ctx, keyID, nil, &subKeyDoc)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}

		err = jwt.Validate(token, subKeyDoc.Value.CKID)
		if err != nil {
			span.RecordError(errors.Wrap(err, "jwt signature validation failed"))
			return nil, err
		}

		return &AuthResult{CCID: ccid}, nil
	}
}
