// Package crypto 只管登录密码：生成盐、算 PBKDF2 哈希。
// 业务字段不再有「客户端派生密钥 + AES 加密」这一层（账户模块已下线，项目也不再存凭证），
// 所以这里保留的就只有密码这一条路。
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"golang.org/x/crypto/pbkdf2"
)

const (
	saltSize = 32
	keySize  = 32
)

// GenerateSalt 生成随机盐值
func GenerateSalt() (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(salt), nil
}

// HashPassword 使用 PBKDF2 生成密码哈希
func HashPassword(password, salt string) string {
	saltBytes, _ := base64.StdEncoding.DecodeString(salt)
	hash := pbkdf2.Key([]byte(password), saltBytes, 10000, keySize, sha256.New)
	return base64.StdEncoding.EncodeToString(hash)
}
