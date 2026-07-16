package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	RedisURL          string
	LiveKitURL        string
	LiveKitAPIKey     string
	LiveKitSecret     string
	TrustProxyHeaders bool
	CookieSecure      bool
	AllowedOrigins    []string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		return Config{}, errors.New("REDIS_URL not set")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	trustProxyHeaders := false
	if value := os.Getenv("TRUST_PROXY_HEADERS"); value != "" {
		var err error
		trustProxyHeaders, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("TRUST_PROXY_HEADERS must be true or false")
		}
	}
	cookieSecure := true
	if value := os.Getenv("COOKIE_SECURE"); value != "" {
		var err error
		cookieSecure, err = strconv.ParseBool(value)
		if err != nil {
			return Config{}, errors.New("COOKIE_SECURE must be true or false")
		}
	}
	allowedOrigins := splitOrigins(os.Getenv("ALLOWED_ORIGINS"))
	if cookieSecure && len(allowedOrigins) == 0 {
		return Config{}, errors.New("ALLOWED_ORIGINS must be set when COOKIE_SECURE is true")
	}

	return Config{
		Port:              port,
		RedisURL:          redisURL,
		LiveKitURL:        os.Getenv("LIVEKIT_URL"),
		LiveKitAPIKey:     os.Getenv("LIVEKIT_API_KEY"),
		LiveKitSecret:     os.Getenv("LIVEKIT_API_SECRET"),
		TrustProxyHeaders: trustProxyHeaders,
		CookieSecure:      cookieSecure,
		AllowedOrigins:    allowedOrigins,
	}, nil
}

func splitOrigins(value string) []string {
	var origins []string
	for _, origin := range strings.Split(value, ",") {
		if origin = strings.TrimSpace(origin); origin != "" && origin != "*" {
			origins = append(origins, origin)
		}
	}
	return origins
}
