package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

var (
	encryptionKey = []byte(getEnv("ENCRYPTION_KEY", "your-32-byte-encryption-key-here"))
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash compares password with hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// EncryptString encrypts a string using AES-GCM
func EncryptString(plaintext string) (string, error) {
	return EncryptBytes([]byte(plaintext))
}

// DecryptString decrypts a string using AES-GCM
func DecryptString(ciphertext string) (string, error) {
	decrypted, err := DecryptBytes(ciphertext)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

// EncryptBytes encrypts byte slice using AES-GCM
func EncryptBytes(data []byte) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptBytes decrypts byte slice using AES-GCM
func DecryptBytes(encodedData string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateAPIKey generates a secure API key
func GenerateAPIKey() (string, error) {
	return GenerateSecureToken(32)
}

// DeriveKey derives a key from password using scrypt
func DeriveKey(password, salt []byte, keyLen int) ([]byte, error) {
	return scrypt.Key(password, salt, 32768, 8, 1, keyLen)
}

// GenerateSalt generates a random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 32)
	_, err := rand.Read(salt)
	return salt, err
}

// HashSHA256 creates SHA256 hash of input
func HashSHA256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

// EncryptFile encrypts file content for secure storage
func EncryptFile(content []byte, userKey string) ([]byte, error) {
	// Use user-specific key for file encryption
	key := sha256.Sum256([]byte(userKey + string(encryptionKey)))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, content, nil), nil
}

// DecryptFile decrypts file content
func DecryptFile(encryptedContent []byte, userKey string) ([]byte, error) {
	// Use same user-specific key for decryption
	key := sha256.Sum256([]byte(userKey + string(encryptionKey)))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedContent) < nonceSize {
		return nil, errors.New("encrypted content too short")
	}

	nonce, ciphertext := encryptedContent[:nonceSize], encryptedContent[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
