package encryption

import (
	"testing"
)

// a4f8754d2a4f0adf12ddaf4bb1c2bc516fe5c18b5572141f32af50903c2a5adb5eba000118abd75080833cd74947beee20049ae0
var data = "test"

// Encrypt encrypts a string
func Test_Encrypt(t *testing.T) {
	encrypted := Encrypt(data)

	decrypted := Decrypt(encrypted)
	println(encrypted)
	if decrypted != data {
		t.Fatalf("decryped string does not match data")
	}
}
