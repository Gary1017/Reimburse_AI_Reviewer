package webhook

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// Verifier handles webhook verification
type Verifier struct {
	verifyToken string
	encryptKey  string
	logger      *zap.Logger
}

// NewVerifier creates a new webhook verifier
func NewVerifier(verifyToken, encryptKey string, logger *zap.Logger) *Verifier {
	return &Verifier{
		verifyToken: verifyToken,
		encryptKey:  encryptKey,
		logger:      logger,
	}
}

// VerifyChallenge handles the initial webhook challenge verification
func (v *Verifier) VerifyChallenge(body []byte) (string, error) {
	var challenge struct {
		Challenge string `json:"challenge"`
		Token     string `json:"token"`
		Type      string `json:"type"`
	}

	if err := json.Unmarshal(body, &challenge); err != nil {
		return "", fmt.Errorf("failed to unmarshal challenge: %w", err)
	}

	if challenge.Type != "url_verification" {
		return "", fmt.Errorf("invalid challenge type: %s", challenge.Type)
	}

	if v.verifyToken != "" && challenge.Token != v.verifyToken {
		return "", fmt.Errorf("invalid verification token")
	}

	return challenge.Challenge, nil
}

// VerifySignature verifies the webhook signature
func (v *Verifier) VerifySignature(timestamp, nonce, signature, body string) bool {
	if v.encryptKey == "" {
		// Signature verification disabled when no encrypt key configured
		return true
	}
	// Concatenate timestamp + nonce + encrypt_key + body
	content := timestamp + nonce + v.encryptKey + body

	// Calculate SHA256
	hash := sha256.Sum256([]byte(content))
	calculated := fmt.Sprintf("%x", hash)

	return calculated == signature
}

// DecryptData decrypts encrypted webhook data
func (v *Verifier) DecryptData(encryptedData string) (string, error) {
	if v.encryptKey == "" {
		return encryptedData, nil // No encryption configured
	}

	// Decode base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Create AES cipher
	key := []byte(v.encryptKey)
	if len(key) != 32 {
		// Pad or truncate to 32 bytes for AES-256
		padded := make([]byte, 32)
		copy(padded, key)
		key = padded
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Use CBC mode
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext = removePKCS7Padding(plaintext)

	return string(plaintext), nil
}

// removePKCS7Padding removes PKCS7 padding
func removePKCS7Padding(data []byte) []byte {
	if len(data) == 0 {
		return data
	}

	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return data
	}

	return data[:len(data)-padding]
}

// ValidateEventType checks if the event type is valid
func (v *Verifier) ValidateEventType(eventType string) bool {
	validTypes := []string{
		"approval_instance",
		"approval.approval_instance",
	}

	for _, valid := range validTypes {
		if strings.Contains(eventType, valid) {
			return true
		}
	}

	return false
}
