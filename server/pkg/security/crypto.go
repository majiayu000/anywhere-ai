package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

// HashPassword 使用bcrypt哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// MD5Hash 计算MD5哈希
func MD5Hash(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SHA256Hash 计算SHA256哈希
func SHA256Hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SHA512Hash 计算SHA512哈希
func SHA512Hash(data string) string {
	hash := sha512.Sum512([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GenerateRandomString 生成随机字符串
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateRandomBytes 生成随机字节
func GenerateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

// AESEncrypt AES加密
func AESEncrypt(plaintext, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	// 生成随机IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// 加密
	stream := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, []byte(plaintext))

	// 将IV和密文组合
	result := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

// AESDecrypt AES解密
func AESDecrypt(ciphertext, key string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", errors.New("密文太短")
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	// 分离IV和密文
	iv := data[:aes.BlockSize]
	ciphertext_bytes := data[aes.BlockSize:]

	// 解密
	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext_bytes))
	stream.XORKeyStream(plaintext, ciphertext_bytes)

	return string(plaintext), nil
}

// GenerateAESKey 生成AES密钥
func GenerateAESKey() (string, error) {
	key := make([]byte, 32) // AES-256
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// ScryptHash 使用scrypt进行密钥派生
func ScryptHash(password, salt string) (string, error) {
	dk, err := scrypt.Key([]byte(password), []byte(salt), 32768, 8, 1, 32)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(dk), nil
}

// GenerateSalt 生成盐值
func GenerateSalt() (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}

// HMACSHA256 计算HMAC-SHA256
func HMACSHA256(data, key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	keyHash := h.Sum(nil)

	h.Reset()
	h.Write(keyHash)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// Base64Encode Base64编码
func Base64Encode(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// Base64Decode Base64解码
func Base64Decode(data string) (string, error) {
	bytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// URLSafeBase64Encode URL安全的Base64编码
func URLSafeBase64Encode(data string) string {
	return base64.URLEncoding.EncodeToString([]byte(data))
}

// URLSafeBase64Decode URL安全的Base64解码
func URLSafeBase64Decode(data string) (string, error) {
	bytes, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// GenerateAPIKey 生成API密钥
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("ak_%s", hex.EncodeToString(bytes)), nil
}

// GenerateSecretKey 生成密钥
func GenerateSecretKey() (string, error) {
	bytes := make([]byte, 64)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("sk_%s", hex.EncodeToString(bytes)), nil
}

// MaskSensitiveData 掩码敏感数据
func MaskSensitiveData(data string, visibleChars int) string {
	if len(data) <= visibleChars*2 {
		return "***"
	}

	start := data[:visibleChars]
	end := data[len(data)-visibleChars:]
	mask := "***"

	return start + mask + end
}

// MaskEmail 掩码邮箱地址
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	// 查找@符号位置
	atIndex := -1
	for i, char := range email {
		if char == '@' {
			atIndex = i
			break
		}
	}

	if atIndex == -1 || atIndex < 2 {
		return "***@***"
	}

	username := email[:atIndex]
	domain := email[atIndex:]

	if len(username) <= 2 {
		return "***" + domain
	}

	maskedUsername := username[:1] + "***" + username[len(username)-1:]
	return maskedUsername + domain
}

// MaskPhone 掩码手机号
func MaskPhone(phone string) string {
	if len(phone) < 7 {
		return "***"
	}

	return phone[:3] + "****" + phone[len(phone)-4:]
}
