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
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pierrre/geohash"
)

var ctx context.Context = context.Background()
var Precision = 8

const (
	Expiry                    = 5 * time.Minute
	MESSAGE_LIMIT_PER_GEOHASH = 10
	REDIS_KEY_PREFIX          = "GEOTALK:"
)

type location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Message struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	InsertedAt time.Time `json:"inserted_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Message    string    `json:"message"`
}

func PutMessage(w http.ResponseWriter, r *http.Request) {
	var msg Message
	err := json.NewDecoder(r.Body).Decode(&msg)

	log.Println("Put /Messages ", msg.Latitude, msg.Longitude, msg.Message)

	if err != nil {
		log.Fatal("Couldn't Decode the Body", err)
		http.Error(w, "Invalid Json", http.StatusBadRequest)
		return
	}
	msg.ExpiresAt = time.Now().Add(Expiry)
	msg.InsertedAt = time.Now()

	defer r.Body.Close()

	hash := GetGeoHash(msg.Latitude, msg.Longitude)

	// fmt.Println("Calculating geohash for %v , %v => %v , message= %v \n", RoundTo4Decimal(msg.Latitude), RoundTo4Decimal(msg.Longitude), hash, msg.Message)

	// fmt.Printf("%v\n", msg)

	w.Header().Set("content-type", "application/json")

	jsonData, _ := json.Marshal(msg)

	_, rerr := rdb.LPush(ctx, REDIS_KEY_PREFIX+hash, jsonData).Result()

	if rerr == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(msg)
		return
	}
	log.Fatalf("Failed to put current geo hash %v into redis %v", hash, rerr)
	http.Error(w, "Failed to Process current post", http.StatusInternalServerError)
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("latitude")
	longStr := r.URL.Query().Get("longitude")

	log.Println("Get /Messages ", latStr, longStr)

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
	currentGeoHash := GetGeoHash(currentLocation.Latitude, currentLocation.Longitude)

	// fmt.Printf("Calculating geohash for %v , %v => %v  \n", currentLocation.Latitude, currentLocation.Longitude, currentGeoHash)

	// neighbours hash
	neighbours, err := geohash.GetNeighbors(currentGeoHash)

	// fmt.Println("neighbours => ", neighbours)

	if err != nil {
		log.Fatal("Error in getting neighbours for geo hash:", currentGeoHash)
	}

	// get neighbours data + currentGeohash data
	keys := []string{neighbours.North, neighbours.East, neighbours.South, neighbours.West, neighbours.NorthEast, neighbours.SouthEast, neighbours.SouthWest, neighbours.NorthWest, currentGeoHash}

	redisResults := GetAllRedisKeyLists(keys)

	sortedMsgLists := ValidateResults(redisResults)

	res := MergeSort(sortedMsgLists)

	// http respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
	return
}

func GetAllRedisKeyLists(keys []string) (res [][]string) {
	pipe := rdb.TxPipeline()
	cmds := make([]*redis.StringSliceCmd, len(keys))

	for i, key := range keys {
		cmds[i] = pipe.LRange(ctx, REDIS_KEY_PREFIX+key, 0, -1)
	}

	_, err := pipe.Exec(ctx)

	if err != nil {
		log.Fatal(err)
	}

	for i, cmd := range cmds {
		vals, err := cmd.Result()
		if err != nil {
			fmt.Println("Error fetching the list for key ", keys[i])
		} else {
			res = append(res, vals)
		}
	}
	return res
}

func GetGeoHash(lat float64, long float64) string {
	return geohash.Encode(RoundTo4Decimal(lat), RoundTo4Decimal(long), Precision)
}

func ValidateResults(redisResults [][]string) (res [][]Message) {

	// redis pipeline for updating the lists by eliminating expired instruments
	pipe := rdb.TxPipeline()
	redisCommands := []redis.Cmder{}

	var v Message

	// iterate over all the geo hashes ( 8 directions + 1 current geo hash)
	for _, msgList := range redisResults {
		if len(msgList) == 0 {
			continue
		}
		temp := []Message{}
		for j, msg := range msgList {

			// provide only top MESSAGE_LIMIT_PER_GEOHASH number of messages per geo hash
			if j > MESSAGE_LIMIT_PER_GEOHASH {
				break
			}
			// unmarshal
			err := json.Unmarshal([]byte(msg), &v)

			// check for expired messages
			if err == nil && v.ExpiresAt.After(time.Now()) {
				temp = append(temp, v)
			} else if err != nil {
				log.Println("error while unmarshalling message list: ", err)
			}
		}
		if len(temp) > 0 {
			// construct command to update current geohash list => remove the expired messages
			geoHash := GetGeoHash(temp[0].Latitude, temp[0].Longitude)
			ttl := time.Until(temp[0].ExpiresAt) // ttl >= 0 as it has been already check in the above validation loop

			// update the whole list to new list with key expiry adjusted to use the first element in the list as list is sorted desc.
			cmd := pipe.LTrim(ctx, REDIS_KEY_PREFIX+geoHash, 0, int64(len(temp)-1))
			expireCmd := pipe.Expire(ctx, REDIS_KEY_PREFIX+geoHash, ttl)
			redisCommands = append(redisCommands, cmd, expireCmd)

			// this is the main required result
			res = append(res, temp)
		} else if len(msgList) > 0 {

			// len(temp) is zero but msgList is not zero => delete this list as all keys may have been expired
			if err := json.Unmarshal([]byte(msgList[0]), &v); err == nil {
				redisCommands = append(redisCommands, pipe.Del(ctx, REDIS_KEY_PREFIX+GetGeoHash(v.Latitude, v.Longitude)))
			}
		}
	}
	// execute the piped commands. silently update all the lists
	go pipe.Exec(ctx)

	return res
}

func Merge(left, right []Message) []Message {
	// merge two sorted arrays
	x, y, k := 0, 0, 0

	res := make([]Message, len(left)+len(right))

	for x < len(left) && y < len(right) {
		if left[x].ExpiresAt.After(right[y].ExpiresAt) {
			res[k] = res[x]
			x++
		} else {
			res[k] = right[y]
			y++
		}
		k++
	}
	res = append(res, left[x:]...)
	res = append(res, right[y:]...)
	return res
}

func MergeSort(msgLists [][]Message) []Message {
	if len(msgLists) == 0 {
		return []Message{}
	}
	if len(msgLists) == 1 {
		return msgLists[0]
	}

	// divide
	mid := len(msgLists) / 2
	left := MergeSort(msgLists[:mid])
	right := MergeSort(msgLists[mid:])

	return Merge(left, right)
}
