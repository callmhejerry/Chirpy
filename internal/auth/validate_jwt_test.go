package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestValidateJwt(t *testing.T) {
	tokenSecret := "token secret"
	userId := uuid.New()
	expiresAt := 5 * time.Second

	jwt, err := MakeJWT(userId, tokenSecret, expiresAt)

	if err != nil {
		t.Fatal("Couldnt make jwt token")
		return
	}

	validatedUserId, err := ValidateJwt(jwt, tokenSecret)

	if err != nil {
		t.Fatal("Failed to validate jwt token")
		return
	}

	if validatedUserId != userId {
		t.Fatal("User ID does not match")
	}
}
