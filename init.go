package main

import (
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
)

var (
    rdb *redis.Client
)
func GetRedisClient() *redis.Client {
    log.Println("Setting up redis client")
	var once sync.Once
	once.Do(func() {
		rdb = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	})
	return rdb
}

func init() {
    GetRedisClient()
}
