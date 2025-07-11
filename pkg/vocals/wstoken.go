package vocals

import (
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
)

const (
	VOCALS_WS_ENDPOINT = "ws://192.168.1.46:8000/v1/stream/conversation"
	TOKEN_EXPIRY_MS    = 10 * 60 * 1000
	API_KEY_MIN_LENGTH = 32
)

func ValidateApiKeyFormat(apiKey string) Result[ValidatedApiKey] {
	if len(apiKey) >= API_KEY_MIN_LENGTH && strings.HasPrefix(apiKey, "vdev_") {
		return Ok(ValidatedApiKey(apiKey))
	}
	return Err[ValidatedApiKey](NewVocalsError("Invalid API key format", "INVALID_API_KEY_FORMAT"))
}

func GetVocalsApiKey() Result[string] {
	apiKey := os.Getenv("VOCALS_DEV_API_KEY")
	if apiKey != "" {
		return Ok(apiKey)
	}
	return Err[string](NewVocalsError("VOCALS_DEV_API_KEY not set", "MISSING_API_KEY"))
}

func GenerateWsTokenFromApiKey(apiKey ValidatedApiKey, userId *string) Result[*WSToken] {
	expiresAt := time.Now().UnixMilli() + TOKEN_EXPIRY_MS

	payload := map[string]interface{}{
		"apiKey": string(apiKey)[:8] + "...",
		"exp":    expiresAt / 1000, // JWT expects seconds
	}
	if userId != nil {
		payload["userId"] = *userId
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(payload))
	tokenString, err := token.SignedString([]byte(apiKey))
	if err != nil {
		return Err[*WSToken](NewVocalsError(err.Error(), "TOKEN_GENERATION_FAILED"))
	}

	return Ok(&WSToken{Token: tokenString, ExpiresAt: expiresAt})
}

func GenerateWsToken() Result[*WSToken] {
	apiKeyResult := GetVocalsApiKey()
	if !apiKeyResult.Success {
		return Err[*WSToken](apiKeyResult.Error)
	}

	validatedResult := ValidateApiKeyFormat(apiKeyResult.Data)
	if !validatedResult.Success {
		return Err[*WSToken](validatedResult.Error)
	}

	return GenerateWsTokenFromApiKey(validatedResult.Data, nil)
}

func GenerateWsTokenWithUserId(userId string) Result[*WSToken] {
	apiKeyResult := GetVocalsApiKey()
	if !apiKeyResult.Success {
		return Err[*WSToken](apiKeyResult.Error)
	}

	validatedResult := ValidateApiKeyFormat(apiKeyResult.Data)
	if !validatedResult.Success {
		return Err[*WSToken](validatedResult.Error)
	}

	return GenerateWsTokenFromApiKey(validatedResult.Data, &userId)
}

func IsTokenExpired(token *WSToken) bool {
	return time.Now().UnixMilli() > token.ExpiresAt
}

func GetTokenTTL(token *WSToken) int {
	ttl := (token.ExpiresAt - time.Now().UnixMilli()) / 1000
	if ttl < 0 {
		return 0
	}
	return int(ttl)
}

func DecodeWsToken(token string, apiKey string) Result[map[string]interface{}] {
	parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		return []byte(apiKey), nil
	})
	if err != nil {
		return Err[map[string]interface{}](NewVocalsError(err.Error(), "TOKEN_DECODE_FAILED"))
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		return Ok(map[string]interface{}(claims))
	}

	return Err[map[string]interface{}](NewVocalsError("Invalid token", "TOKEN_DECODE_FAILED"))
}

func GetWsEndpoint() string {
	return VOCALS_WS_ENDPOINT
}

func GetTokenExpiryMs() int {
	return TOKEN_EXPIRY_MS
}

func init() {
	_ = godotenv.Load()
}
