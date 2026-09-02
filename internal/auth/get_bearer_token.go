package auth

import (
	"errors"
	"net/http"
	"strings"
)

func GetBearerToken(headers http.Header) (string, error) {
	token := headers.Get("Authorization")

	if token == "" {
		return "", errors.New("unable to find authorization token")
	}

	tokenParts := strings.Split(token, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return "", errors.New("invalid authorization token format")
	}
	return tokenParts[1], nil
}
