package security

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey         string        `mapstructure:"secret_key" yaml:"secret_key"`
	Issuer            string        `mapstructure:"issuer" yaml:"issuer"`
	Expiration        time.Duration `mapstructure:"expiration" yaml:"expiration"`
	RefreshExpiration time.Duration `mapstructure:"refresh_expiration" yaml:"refresh_expiration"`
}

// DefaultJWTConfig 默认JWT配置
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SecretKey:         "your-secret-key",
		Issuer:            "gin-starter",
		Expiration:        24 * time.Hour,
		RefreshExpiration: 7 * 24 * time.Hour,
	}
}

// Claims JWT声明
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager JWT管理器
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager 创建JWT管理器
func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{
		config: config,
	}
}

// GenerateToken 生成访问令牌
func (j *JWTManager) GenerateToken(userID, username, email, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.config.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.Expiration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// GenerateRefreshToken 生成刷新令牌
func (j *JWTManager) GenerateRefreshToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		Issuer:    j.config.Issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.config.RefreshExpiration)),
		NotBefore: jwt.NewNumericDate(now),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// ValidateToken 验证令牌
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("无效的签名方法")
		}
		return []byte(j.config.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的令牌")
}

// ValidateRefreshToken 验证刷新令牌
func (j *JWTManager) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("无效的签名方法")
		}
		return []byte(j.config.SecretKey), nil
	})

	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims.Subject, nil
	}

	return "", errors.New("无效的刷新令牌")
}

// RefreshToken 刷新令牌
func (j *JWTManager) RefreshToken(refreshTokenString string, userID, username, email, role string) (string, string, error) {
	// 验证刷新令牌
	validUserID, err := j.ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return "", "", err
	}

	if validUserID != userID {
		return "", "", errors.New("用户ID不匹配")
	}

	// 生成新的访问令牌和刷新令牌
	accessToken, err := j.GenerateToken(userID, username, email, role)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := j.GenerateRefreshToken(userID)
	if err != nil {
		return "", "", err
	}

	return accessToken, newRefreshToken, nil
}

// ExtractTokenFromHeader 从请求头中提取令牌
func ExtractTokenFromHeader(authHeader string) string {
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// GenerateTokenPair 生成令牌对
func (j *JWTManager) GenerateTokenPair(userID, username, email, role string) (*TokenPair, error) {
	accessToken, err := j.GenerateToken(userID, username, email, role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := j.GenerateRefreshToken(userID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(j.config.Expiration.Seconds()),
	}, nil
}
