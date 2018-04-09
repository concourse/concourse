package token

import (
	"context"
	"errors"

	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"
)

type VerifiedClaims struct {
	Sub         string
	Email       string
	Name        string
	UserID      string
	UserName    string
	ConnectorID string
	Groups      []string
}

//go:generate counterfeiter . Verifier
type Verifier interface {
	Verify(context.Context, *oauth2.Token) (*VerifiedClaims, error)
}

func NewVerifier(clientID, issuerURL string) *verifier {
	return &verifier{
		ClientID:  clientID,
		IssuerURL: issuerURL,
	}
}

type verifier struct {
	ClientID  string
	IssuerURL string
}

func (self *verifier) Verify(ctx context.Context, token *oauth2.Token) (*VerifiedClaims, error) {

	if self.ClientID == "" {
		return nil, errors.New("Missing client id")
	}

	if self.IssuerURL == "" {
		return nil, errors.New("Missing issuer")
	}

	if ctx == nil {
		return nil, errors.New("Missing context")
	}

	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("Missing id_token")
	}

	provider, err := oidc.NewProvider(ctx, self.IssuerURL)
	if err != nil {
		return nil, err
	}

	providerVerifier := provider.Verifier(&oidc.Config{
		ClientID: self.ClientID,
	})

	verifiedToken, err := providerVerifier.Verify(ctx, idToken)
	if err != nil {
		return nil, err
	}

	type Federated struct {
		ConnectorID string `json:"connector_id"`
		UserID      string `json:"user_id"`
		UserName    string `json:"user_name"`
	}

	type Claims struct {
		Sub       string    `json:"sub"`
		Email     string    `json:"email"`
		Name      string    `json:"name"`
		Groups    []string  `json:"groups"`
		Federated Federated `json:"federated_claims"`
	}

	var claims Claims
	verifiedToken.Claims(&claims)

	return &VerifiedClaims{
		Sub:         claims.Sub,
		Email:       claims.Email,
		Name:        claims.Name,
		UserID:      claims.Federated.UserID,
		UserName:    claims.Federated.UserName,
		ConnectorID: claims.Federated.ConnectorID,
		Groups:      claims.Groups,
	}, nil
}
