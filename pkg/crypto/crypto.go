package crypto


import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// Encrypt mengenskripsi data dengan key yang diberikan.
func Encrypt(data, key string) (string, error) {
	key = FixEncryptionKey(key)

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	plaintext := []byte(data)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt mendekripsi data dengan key yang diberikan.
func Decrypt(data, key string) (string, error) {
	key = FixEncryptionKey(key)

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	ciphertext, _ := base64.StdEncoding.DecodeString(data)
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// FixEncryptionKey memastikan key memiliki panjang 32 byte.
func FixEncryptionKey(key string) string {
	if len(key) < 32 {
		return key + strings.Repeat("0", 32-len(key))
	}
	return key[:32]
}
