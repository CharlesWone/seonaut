// utils/param_encrypt.go
package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// 固定 32 字节 AES-256 密钥（复杂、高熵、安全）
// 对应 64 个十六进制字符
var aesKey = []byte{
	0x4a, 0x9f, 0x8e, 0x2c, 0x1d, 0x7b, 0x3a, 0x6f,
	0x5e, 0x4d, 0x8c, 0x9b, 0x0a, 0x1f, 0x2e, 0x3d,
	0x7c, 0x6b, 0x5a, 0x4f, 0x9e, 0x8d, 0x7c, 0x6b,
	0x5a, 0x4f, 0x3e, 0x2d, 0x1c, 0x0b, 0x9a, 0x8f,
}

// EncryptParam 加密任意参数字符串
func EncryptParam(plain string) (string, error) {
	block, err := aes.NewCipher(aesKey)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptParam 解密 token → 原始参数字符串
func DecryptParam(token string) (string, error) {
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", errors.New("invalid token format")
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("token too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("invalid or tampered token")
	}
	return string(plain), nil
}
