package auth

import "testing"

func TestCheckPasswordHash(t *testing.T) {
	password := "Test123!"
	hashedPassword := "$argon2id$v=19$m=65536,t=1,p=8$XfwoClNrtQrAdLWjO5CTbQ$XFUzUnQL2Jk8Jf6A1ZYuKu+DXWiXa2J/YTjpkEyFX+M"

	// if err != nil {
	// 	t.Logf("failed to hash passoword %v\n", err)
	// }

	passwordMatch, err := CheckPasswordHash(password, hashedPassword)

	if err != nil {
		t.Logf("failed to check password hash %v\n", err)
	}
	if !passwordMatch {
		t.Logf("Password does not match\n")
	}
}
