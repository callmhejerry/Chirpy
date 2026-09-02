package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetApiKey(header http.Header) (string, error) {
	authorizationValue := header.Get("Authorization")

	authorizationValueParts := strings.Split(authorizationValue, " ")

	if len(authorizationValueParts) != 2 || authorizationValueParts[0] != "ApiKey" {
		return "", fmt.Errorf("Malformed or invalid api key")
	}
	return authorizationValueParts[1], nil
}
