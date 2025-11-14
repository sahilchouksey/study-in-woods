package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("token has expired")
	ErrInvalidClaims = errors.New("invalid token claims")
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret        string
	Expiry        time.Duration
	RefreshExpiry time.Duration
	Issuer        string
}

// Claims represents JWT claims
type Claims struct {
	UserID       uint   `json:"user_id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	TokenType    string `json:"token_type"`    // "access" or "refresh"
	TokenVersion int    `json:"token_version"` // For invalidating all tokens
	jwt.RegisteredClaims
}

// JWTManager handles JWT token operations
type JWTManager struct {
	config JWTConfig
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(config JWTConfig) *JWTManager {
	return &JWTManager{
		config: config,
	}
}

// GenerateAccessToken generates a new access token with JTI
func (j *JWTManager) GenerateAccessToken(userID uint, email string, role string, tokenVersion int) (string, string, error) {
	now := time.Now()
	expiresAt := now.Add(j.config.Expiry)
	jti := uuid.New().String()

	claims := Claims{
		UserID:       userID,
		Email:        email,
		Role:         role,
		TokenType:    "access",
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    j.config.Issuer,
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(j.config.Secret))
	return signedToken, jti, err
}

// GenerateRefreshToken generates a new refresh token with JTI
func (j *JWTManager) GenerateRefreshToken(userID uint, email string, role string, tokenVersion int) (string, string, error) {
	now := time.Now()
	expiresAt := now.Add(j.config.RefreshExpiry)
	jti := uuid.New().String()

	claims := Claims{
		UserID:       userID,
		Email:        email,
		Role:         role,
		TokenType:    "refresh",
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    j.config.Issuer,
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(j.config.Secret))
	return signedToken, jti, err
}

// ValidateToken validates a JWT token and returns claims
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(j.config.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// ExtractClaims extracts claims from token without validation (for debugging)
func (j *JWTManager) ExtractClaims(tokenString string) (*Claims, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}

// RefreshAccessToken generates a new access token from a valid refresh token
func (j *JWTManager) RefreshAccessToken(refreshToken string, tokenVersion int) (string, string, error) {
	claims, err := j.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	if claims.TokenType != "refresh" {
		return "", "", ErrInvalidToken
	}

	return j.GenerateAccessToken(claims.UserID, claims.Email, claims.Role, tokenVersion)
}

// GetTokenExpiry returns the expiry time of a token
func (j *JWTManager) GetTokenExpiry(tokenString string) (time.Time, error) {
	claims, err := j.ExtractClaims(tokenString)
	if err != nil {
		return time.Time{}, err
	}

	if claims.ExpiresAt == nil {
		return time.Time{}, errors.New("token has no expiry")
	}

	return claims.ExpiresAt.Time, nil
}

// IsTokenExpired checks if a token is expired
func (j *JWTManager) IsTokenExpired(tokenString string) bool {
	expiry, err := j.GetTokenExpiry(tokenString)
	if err != nil {
		return true
	}
	return time.Now().After(expiry)
}

// GetJTI extracts the JTI (token ID) from a token
func (j *JWTManager) GetJTI(tokenString string) (string, error) {
	claims, err := j.ExtractClaims(tokenString)
	if err != nil {
		return "", err
	}
	return claims.ID, nil
}
