package main

import (
	"log"
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
		redis_address := os.Getenv("REDIS_URL")
		if redis_address == "" {
			log.Fatal("REDIS_URL not provided..")
		}
		rdb = redis.NewClient(&redis.Options{
			Addr: redis_address,
		})
	})
	return rdb
}

func init() {
	allowedOrigin = os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:3000"
	}
    GetRedisClient()
}
