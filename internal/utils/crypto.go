package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

var (
	// 默认加密密钥
	defaultKey = []byte("12345678901234567890123456789012")
	// 当前使用的加密密钥
	currentKey = defaultKey
)

// 用于测试的函数
func setEncryptionKey(key []byte) (restore func()) {
	oldKey := currentKey
	currentKey = key
	return func() {
		currentKey = oldKey
	}
}

// EncryptPassword 加密密码
func EncryptPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("密码不能为空")
	}

	block, err := aes.NewCipher(currentKey)
	if err != nil {
		return "", err
	}

	plaintext := []byte(password)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptPassword 解密密码
func DecryptPassword(encrypted string) (string, error) {
	if encrypted == "" {
		return "", fmt.Errorf("密文不能为空")
	}

	block, err := aes.NewCipher(currentKey)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("密文太短")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

// IsEncrypted 检查密码是否已加密, 注意: 空密码不是有效密码
func IsEncrypted(password string) bool {
	// 1. 基本长度检查
	if len(password) < 16 { // 至少需要IV(16字节)的base64编码长度
		return false
	}

	// 2. 尝试base64解码
	decoded, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		return false
	}

	// 3. 检查解码后的长度
	if len(decoded) <= aes.BlockSize { // 至少需要一个IV块
		return false
	}

	// 4. 检查总长度是否合理（不超过一个合理的密码长度的加密结果）
	maxEncryptedLen := 2048 // 假设原始密码不会超过2048字节
	return len(decoded) <= maxEncryptedLen
}
