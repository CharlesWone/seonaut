// utils/param_encrypt.go
package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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

// EncryptMap 将 map[string]any 加密为一个 token 字符串
// 支持任意可 JSON 序列化的值（string/int/bool/nil/float/struct/slice/map 等）
func EncryptMap(data map[string]any) (string, error) {
	// 1. 先序列化为规范的 JSON（排序键、确定性）
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	// 2. 用原来的 AES-GCM 加密流程加密 JSON 字节
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, jsonBytes, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptMap 将 token 解密为 map[string]any
func DecryptMap(token string) (map[string]any, error) {
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return nil, errors.New("invalid token format")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("token too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plainBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("invalid or tampered token")
	}

	var result map[string]any
	if err := json.Unmarshal(plainBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

//// ====================== 可选：如果你只想支持 map[string]string（更严格） ======================
//
//func EncryptStringMap(data map[string]string) (string, error) {
//	// 转为 map[string]any 再走通用流程
//	anyMap := make(map[string]any, len(data))
//	for k, v := range data {
//		anyMap[k] = v
//	}
//	return EncryptMap(anyMap)
//}
//
//func DecryptStringMap(token string) (map[string]string, error) {
//	m, err := DecryptMap(token)
//	if err != nil {
//		return nil, err
//	}
//	result := make(map[string]string, len(m))
//	for k, v := range m {
//		if str, ok := v.(string); ok {
//			result[k] = str
//		} else {
//			return nil, errors.New("map contains non-string value")
//		}
//	}
//	return result, nil
//}
//
//// ====================== 额外建议：确定性 JSON（可选更严格） ======================
//// 如果你非常在意相同 map 每次加密后 token 完全一样（便于缓存/对比），可以引入确定性 marshal：
//// go get github.com/neilotoole/jsoncolor 或自己实现按 key 排序 marshal。
//// 但一般不建议，因为会牺牲随机 nonce 的安全性优势（nonce 已经随机了，密文自然不同）。

/// 使用示例
//func main() {
//	params := map[string]any{
//		"user_id":   12345,
//		"role":      "admin",
//		"exp":       1732156800,
//		"features":  []string{"beta", "pro"},
//		"metadata":  map[string]any{"theme": "dark"},
//	}
//
//	token, err := EncryptMap(params)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println("token:", token)
//
//	decrypted, err := DecryptMap(token)
//	if err != nil {
//		panic(err)
//	}
//	fmt.Printf("decrypted: %+v\n", decrypted)
//}
