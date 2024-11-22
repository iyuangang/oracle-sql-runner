package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"
)

const (
	keyFile  = ".key"
	saltFile = ".salt"
	keyLen   = 32
	saltLen  = 32
	nonceLen = 12
)

type Crypto struct {
	key  []byte
	salt []byte
}

func NewCrypto() (*Crypto, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	c := &Crypto{}
	if err := c.loadOrGenerateKey(configDir); err != nil {
		return nil, err
	}
	if err := c.loadOrGenerateSalt(configDir); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Crypto) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)
	encrypted := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (c *Crypto) Decrypt(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	if len(data) < nonceLen {
		return "", errors.New("invalid encrypted data")
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := data[:nonceLen]
	ciphertext := data[nonceLen:]

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (c *Crypto) loadOrGenerateKey(configDir string) error {
	keyPath := filepath.Join(configDir, keyFile)

	// 尝试加载现有密钥
	if key, err := os.ReadFile(keyPath); err == nil {
		c.key = key
		return nil
	}

	// 生成新密钥
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return err
	}

	// 保存密钥
	if err := os.WriteFile(keyPath, key, 0o600); err != nil {
		return err
	}

	c.key = key
	return nil
}

func (c *Crypto) loadOrGenerateSalt(configDir string) error {
	saltPath := filepath.Join(configDir, saltFile)

	// 尝试加载现有盐值
	if salt, err := os.ReadFile(saltPath); err == nil {
		c.salt = salt
		return nil
	}

	// 生成新盐值
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}

	// 保存盐值
	if err := os.WriteFile(saltPath, salt, 0o600); err != nil {
		return err
	}

	c.salt = salt
	return nil
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".oracle-sql-runner")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return "", err
	}

	return configDir, nil
}

func deriveKey(password []byte, salt []byte) ([]byte, error) {
	return scrypt.Key(password, salt, 32768, 8, 1, keyLen)
}
