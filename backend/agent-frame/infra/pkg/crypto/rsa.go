package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
)

var hashSelector = sha256.New()

// MarshalRSAPublicKeyPEM marshals an RSA public key to PEM format
func MarshalRSAPublicKeyPEM(pub *rsa.PublicKey) ([]byte, error) {
	pkixBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixBytes,
	}
	return pem.EncodeToMemory(block), nil
}

// MarshalRSAPrivateKeyPEM marshals an RSA private key to PEM format
func MarshalRSAPrivateKeyPEM(priv *rsa.PrivateKey) ([]byte, error) {
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	return pem.EncodeToMemory(block), nil
}

// RSAKeyPair holds RSA private and public keys
type RSAKeyPair struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// GenerateRSAKeyPair generates a new RSA key pair
func GenerateRSAKeyPair(bits int) (*RSAKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	return &RSAKeyPair{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
	}, nil
}

// ParsePrivateKey parses a PEM-encoded RSA private key
func ParsePrivateKey(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ParsePublicKey parses a PEM-encoded RSA public key
func ParsePublicKey(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pubKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	return pubKey, nil
}

// EncryptRSAOAEP encrypts data with RSA-OAEP
func EncryptRSAOAEP(plaintext []byte, pub *rsa.PublicKey) ([]byte, error) {
	return rsa.EncryptOAEP(hashSelector, rand.Reader, pub, plaintext, nil)
}

// DecryptRSAOAEP decrypts data with RSA-OEAP
func DecryptRSAOAEP(ciphertext []byte, priv *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(hashSelector, rand.Reader, priv, ciphertext, nil)
}

// EncryptRSAOAEPBase64 encrypts and returns base64-encoded result
func EncryptRSAOAEPBase64(plaintext []byte, pub *rsa.PublicKey) (string, error) {
	ciphertext, err := EncryptRSAOAEP(plaintext, pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptRSAOAEPBase64 decrypts base64-encoded ciphertext
func DecryptRSAOAEPBase64(ciphertextB64 string, priv *rsa.PrivateKey) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	return DecryptRSAOAEP(ciphertext, priv)
}

// GenerateAESKey generates a random AES key
func GenerateAESKey(length int) ([]byte, error) {
	key := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// EncryptAES encrypts data with AES-GCM
func EncryptAES(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
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
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAES decrypts data with AES-GCM
func DecryptAES(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// EncryptAESGCMBase64 encrypts data with AES-GCM and returns base64
func EncryptAESGCMBase64(plaintext []byte, key []byte) (string, error) {
	ciphertext, err := EncryptAES(plaintext, key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAESGCMBase64 decrypts base64-encoded AES-GCM ciphertext
func DecryptAESGCMBase64(ciphertextB64 string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	return DecryptAES(ciphertext, key)
}
