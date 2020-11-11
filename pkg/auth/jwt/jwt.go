package jwt

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	AlgHS256 = "HS256"
	TypJWT   = "JWT"
)

type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type Claims struct {
	Issuer         string `json:"iss,omitempty"`
	Subject        string `json:"sub,omitempty"`
	Audience       string `json:"aud,omitempty"`
	ExpirationTime int64  `json:"exp,omitempty"`
	NotBefore      int64  `json:"nbf,omitempty"`
	IssuedAt       int64  `json:"iat,omitempty"`
	JwtID          string `json:"jti,omitempty"`
	TokenType      string `json:"tkt,omitempty"`
}

type Token struct {
	Header  Header
	Payload Claims
}

func (c *Claims) Verify() error {
	// TODO: More verification
	if time.Unix(c.ExpirationTime, 0).Before(time.Now()) {
		return fmt.Errorf("token expired: %s > %d", time.Now(), c.ExpirationTime)
	}

	return nil
}

func New(algorithm string, payload Claims) Token {
	t := Token{}
	header := Header{Alg: algorithm, Typ: TypJWT}
	t.Header = header
	t.Payload = payload

	return t
}

func (t *Token) Sign(secret string) (string, error) {
	jwt := ""

	headerJSON, err := json.Marshal(t.Header)
	if err != nil {
		return jwt, err
	}

	payloadJSON, err := json.Marshal(t.Payload)
	if err != nil {
		return jwt, err
	}

	headerString := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadString := base64.RawURLEncoding.EncodeToString(payloadJSON)

	jwt = headerString + "." + payloadString

	hash := hmac.New(sha256.New, []byte(secret))
	_, err = hash.Write([]byte(jwt))
	if err != nil {
		return "", err
	}

	signed := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))

	jwt += "." + signed

	return jwt, nil
}

func Verify(token string, secret string, algorithm string, payload Claims) (*Token, error) {
	if token == "" {
		return nil, errors.New("token is empty")
	}

	if len(strings.Split(token, ".")) != 3 {
		return nil, errors.New("no 3 part token structure")
	}

	const headerPart = 0
	const payloadPart = 1
	const signaturePart = 2

	tokenParts := strings.Split(token, ".")
	decodedHeader, err := base64.RawURLEncoding.DecodeString(tokenParts[headerPart])
	if err != nil {
		return nil, err
	}

	if !json.Valid(decodedHeader) {
		return nil, errors.New("header json not valid")
	}

	header := Header{}
	err = json.Unmarshal(decodedHeader, &header)

	if err != nil {
		return nil, err
	}

	if header.Typ != TypJWT || header.Alg != algorithm {
		return nil, errors.New("incompatible token")
	}

	decodedPayload, err := base64.RawURLEncoding.DecodeString(tokenParts[payloadPart])

	if err != nil {
		return nil, err
	}

	hash := hmac.New(sha256.New, []byte(secret))
	_, err = hash.Write([]byte(tokenParts[headerPart] + "." + tokenParts[payloadPart]))
	if err != nil {
		return nil, err
	}

	checkHash := base64.RawURLEncoding.EncodeToString(hash.Sum(nil))
	if !bytes.Equal([]byte(checkHash), []byte(tokenParts[signaturePart])) {
		return nil, errors.New("invalid signature")
	}

	// Check payload functional requirements
	if !json.Valid(decodedPayload) {
		return nil, errors.New("payload json not valid")
	}

	err = json.Unmarshal(decodedPayload, &payload)
	if err != nil {
		return nil, errors.New("payload json not valid")
	}

	err = payload.Verify()
	if err != nil {
		return nil, err
	}

	decodedToken := New(algorithm, payload)

	return &decodedToken, nil
}
