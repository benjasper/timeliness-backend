package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
)

// Decrypt decrypt a string
func Decrypt(data string) string {
	key := []byte(createHash(getSecret()))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonceSize := gcm.NonceSize()
	dataDecoded, _ := hex.DecodeString(data)
	nonce, ciphertext := dataDecoded[:nonceSize], dataDecoded[nonceSize:]
	plaintext, err := gcm.Open(nil, []byte(nonce), []byte(ciphertext), nil)
	if err != nil {
		return ""
	}

	return string(plaintext)
}

// Encrypt encrypts a string
func Encrypt(data string) string {
	block, _ := aes.NewCipher([]byte(createHash(getSecret())))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(data), nil)

	return hex.EncodeToString(ciphertext)
}

func getSecret() string {
	secret := os.Getenv("SECRET")
	if secret == "" {
		return "Secret-dev-key-that-is-32-bytes-long"
	}

	return secret
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}
