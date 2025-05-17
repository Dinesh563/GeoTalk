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
	log.Println("Handling PutMessage Route")

	var msg Message
	err := json.NewDecoder(r.Body).Decode(&msg)

	if err != nil {
		log.Fatal("Couldn't Decode the Body", err)
		http.Error(w, "Invalid Json", http.StatusBadRequest)
		return
	}
	msg.ExpiresAt = time.Now().Add(Expiry)
	msg.InsertedAt = time.Now()

	defer r.Body.Close()

	hash := geohash.Encode(RoundTo4Decimal(msg.Latitude), RoundTo4Decimal(msg.Longitude), Precision)

	fmt.Printf("Calculating geohash for %v , %v => %v , message= %v \n", RoundTo4Decimal(msg.Latitude), RoundTo4Decimal(msg.Longitude), hash, msg.Message)

	// fmt.Printf("%v\n", msg)

	w.Header().Set("content-type", "application/json")

	jsonData, _ := json.Marshal(msg)

	_, rerr := rdb.LPush(ctx, REDIS_KEY_PREFIX+hash, jsonData).Result()

	if rerr == nil {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"data": "ok"})
		return
	}
	log.Fatalf("Failed to put current geo hash %v into redis %v", hash, rerr)
	http.Error(w, "Failed to Process current post", http.StatusInternalServerError)
}

func GetMessages(w http.ResponseWriter, r *http.Request) {
	// log.Println("Get Messages Handle")

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

	currentLocation := location{Latitude: RoundTo4Decimal(lat), Longitude: RoundTo4Decimal(long)}

	// get current geohash
	currentGeoHash := geohash.Encode(currentLocation.Latitude, currentLocation.Longitude, Precision)

	fmt.Printf("Calculating geohash for %v , %v => %v  \n", currentLocation.Latitude, currentLocation.Longitude, currentGeoHash)

	// neighbours hash
	neighbours, err := geohash.GetNeighbors(currentGeoHash)

	fmt.Println("neighbours => ", neighbours)

	if err != nil {
		log.Fatal("Error in getting neighbours for geo hash:", currentGeoHash)
	}

	// get neighbours data + currentGeohash data
	keys := []string{neighbours.North, neighbours.East, neighbours.South, neighbours.West, neighbours.NorthEast, neighbours.SouthEast, neighbours.SouthWest, neighbours.NorthWest, currentGeoHash}

	redisResults := GetAllRedisKeyLists(keys)

	sortedMsgLists := UnMarshal(redisResults)

	res := Merge(sortedMsgLists)

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

func UnMarshal(redisResults [][]string) [][]Message {
	var v Message
	res := make([][]Message, len(redisResults))
	for _, msgList := range redisResults {
		temp := []Message{}
		for j, msg := range msgList {
			if j > MESSAGE_LIMIT_PER_GEOHASH {
				break
			}
			err := json.Unmarshal([]byte(msg), &v)

			if err == nil && v.ExpiresAt.After(time.Now()) {
				temp = append(temp, v)
			} else if err != nil {
				log.Println("error while unmarshalling message list: ", err)
			}
		}
		res = append(res, temp)
	}
	return res
}

func Merge(msgLists [][]Message) (res []Message) {
	if len(msgLists) == 0 {
		return res
	}

	res = msgLists[0]

	for i := 1; i < len(msgLists); i++ {

		// merge two sorted arrays
		x, y := 0, 0

		temp := []Message{}

		for x < len(res) && y < len(msgLists[i]) {
			if res[x].ExpiresAt.After(msgLists[i][y].ExpiresAt) {
				temp = append(temp, res[x])
				x++
			} else {
				temp = append(temp, msgLists[i][y])
				y++
			}
		}
		//append the remaining elements
		for x < len(res) {
			temp = append(temp, res[x])
			x++
		}

		for y < len(msgLists[i]) {
			temp = append(temp, msgLists[i][y])
			y++
		}
		res = temp
	}
	return res
}
