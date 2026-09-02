package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func ValidateJwt(tokenString string, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, jwt.Keyfunc(func(t *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	}))

	if err != nil {
		return uuid.UUID{}, err
	}
	subject, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}
	return uuid.Parse(subject)
}
