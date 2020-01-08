package token

import (
	"errors"

	"golang.org/x/oauth2"
)

func NewTokenSource(source oauth2.TokenSource) *idTokenSource {
	return &idTokenSource{source}
}

type idTokenSource struct {
	oauth2.TokenSource
}

func (c *idTokenSource) Token() (*oauth2.Token, error) {
	token, err := c.TokenSource.Token()
	if err != nil {
		return nil, err
	}

	return UseIDToken(token)
}

func UseIDToken(token *oauth2.Token) (*oauth2.Token, error) {

	idToken := token.Extra("id_token")
	idTokenString, ok := idToken.(string)
	if !ok {
		return nil, errors.New("invalid id_token")
	}

	token.AccessToken = idTokenString
	return token, nil
}
