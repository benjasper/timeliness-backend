package jwt

import (
	"strings"
	"testing"
	"time"
)

func TestToken_New(t *testing.T) {
	payload := Claims{ExpirationTime: time.Now().Unix()}
	New(AlgHS256, payload)
}

func TestToken_Sign(t *testing.T) {
	payload := Claims{ExpirationTime: time.Now().Unix()}
	token := New(AlgHS256, payload)
	tokenString, err := token.Sign("secret")
	if err != nil {
		t.Error(err)
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		t.Error("JWT does not have 3 parts")
	}
}

func TestVerify(t *testing.T) {
	payload := Claims{ExpirationTime: time.Now().Unix() - 100, TokenType: TokenTypeAccess}
	token := New(AlgHS256, payload)
	tokenString, err := token.Sign("secret")
	if err != nil {
		t.Error(err)
	}

	newClaims := Claims{}

	verifiedToken, err := Verify(tokenString, TokenTypeAccess, "secret", AlgHS256, newClaims)
	if err == nil && verifiedToken != nil {
		t.Error(err)
	}
}
