package main

import (
	"log"
	"net/url"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
)

var (
	rdb *redis.Client
)

var allowedOrigin string

func GetRedisClient() *redis.Client {
	log.Println("Setting up redis client")
	var once sync.Once
	once.Do(func() {
		rawUrl := os.Getenv("REDIS_URL")
		if rawUrl == "" {
			log.Fatal("REDIS_URL not provided")
		}

		parsed, err := url.Parse(rawUrl)
		if err != nil {
			log.Fatalf("Invalid REDIS_URL: %v", err)
		}

		rdb = redis.NewClient(&redis.Options{
			Addr:     parsed.Host,              // host:port
			Username: parsed.User.Username(),   // e.g. "default"
			Password: getRedisPassword(parsed), // from parsed.User
		})
	})
	return rdb
}

func getRedisPassword(u *url.URL) string {
	if pwd, ok := u.User.Password(); ok {
		return pwd
	}
	return ""
}

func init() {
	allowedOrigin = os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}
	GetRedisClient()
}
