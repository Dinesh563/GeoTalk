package main

// curl --location 'localhost:8080/PutMessage' \
// --header 'Content-Type: application/json' \
// --data '{
//     "latitude": 12.9753,
//     "longitude": 77.591,
//     "message": "HI there"
// }'

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/pierrre/geohash"
)

var ctx context.Context = context.Background()
var Precision = 8

type location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func PutMessage(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling PutMessage Route")

	var loc location
	err := json.NewDecoder(r.Body).Decode(&loc)

	if err != nil {
		log.Fatal("Couldn't Decode the Body", err)
		http.Error(w, "Invalid Json", http.StatusBadRequest)
		return
	}

	hash := geohash.Encode(loc.Latitude, loc.Longitude, Precision)

	neighbours, err := geohash.GetNeighbors(hash)
	if err != nil {
		log.Fatal("Error in getting neighbours for geo hash:", hash)
	}
	fmt.Printf("current hash: %v, Neighbours: %+v", hash, neighbours)

	defer r.Body.Close()

	fmt.Printf("%v\n", loc)

	w.Header().Set("content-type", "application/json")

	rerr := rdb.Set(ctx, hash, "HI from: "+hash, 0)
	if rerr != nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"data": "ok"})
		return
	}
	log.Fatalf("Failed to put current geo hash %v into redis %v", hash, rerr)
	http.Error(w, "Failed to Process current post", http.StatusInternalServerError)
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	log.Println("Get Messages Handle")

	latStr := r.URL.Query().Get("latitude")
	longStr := r.URL.Query().Get("longitude")

	if latStr == "" || longStr == "" {
		http.Error(w, "Missing latitude or longitude", http.StatusBadRequest)
		return
	}

	lat, err1 := strconv.ParseFloat(latStr, 64)
	long, err2 := strconv.ParseFloat(longStr, 64)

	if err1 != nil || err2 != nil {
		http.Error(w, "Invalid latitude or longitude", http.StatusBadRequest)
		return
	}

	currentLocation := location{Latitude: lat, Longitude: long}

	// get current geohash
	currentGeoHash := geohash.Encode(currentLocation.Latitude, currentLocation.Longitude, Precision)

	// neighbours hash
	neighbours, err := geohash.GetNeighbors(currentGeoHash)

	if err != nil {
		log.Fatal("Error in getting neighbours for geo hash:", currentGeoHash)
	}

	// get neighbours data + currentGeohash data
	keys := []string{neighbours.North, neighbours.East, neighbours.South, neighbours.West, neighbours.NorthEast, neighbours.SouthEast, neighbours.SouthWest, neighbours.NorthWest, currentGeoHash}

	result, err := rdb.MGet(ctx, keys...).Result()

	if err != nil {
		log.Fatal("Error fetching keys from redis, err :", err)
	}
	// http respond
	json.NewEncoder(w).Encode(result)
	w.WriteHeader(http.StatusOK)
	return
}
