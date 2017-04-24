package main

import (
	"fmt"
	"os"
	"strings"

	"net/http"

	"log"
	"strconv"
	"time"

	"github.build.ge.com/212419672/cf-service-tester/cfServiceDiscovery"

	"encoding/json"

	"github.com/cloudfoundry-community/go-cfenv"
	"gopkg.in/redis.v3"
)

var redisClient *redis.Client
var redisHost, redisPass, redisPort string
var myService cfServiceDiscovery.ServiceDescriptor
var serviceName string
var planName string

// Test redis by:
// 1)  generating a k:v pair,
// 2)  inserting into Redis,
// 3)  querying Redis for the key,
// 4)  and comparing the valueIn with the valueOut.
func testRedis(w http.ResponseWriter, req *http.Request) {
	log.Print("in testRedis()")
	if len(redisHost) == 0 {
		fmt.Fprint(w, "I'm not bound to a Redis instance!  Please bind me!\n")
		return
	}
	timeStamp := strconv.FormatInt(time.Now().Unix(), 10)
	key := "key_" + timeStamp
	valueIn := "val_" + timeStamp

	err := redisClient.Set(key, valueIn, 0).Err()
	if err != nil {
		log.Printf("Could not set redis value %v::%v, err= %v", key, valueIn, err)
		fmt.Fprintf(w, "Could not set redis value, for key: %v, value: %v\n", key, valueIn)
		return
	}

	valueOut, err := redisClient.Get(key).Result()
	if err != nil {
		log.Printf("Could not get redis value %v::%v, err= %v", key, valueIn, err)
		fmt.Fprintf(w, "Could not get redis value, for key %v\n", key)
		return
	}

	if strings.EqualFold(valueIn, valueOut) {
		log.Printf("Looks ok:  valueIn: %v == valueOut: %v\n", valueIn, valueOut)
		fmt.Fprintf(w, "Looks ok:  valueIn: %v, valueOut: %v\n", valueIn, valueOut)
		fmt.Fprintf(w, "%v working correctly!", myService.ServiceName)
		return
	}

}

// Return my service descriptor metadata
func serviceDescriptor(w http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(&myService)
	if err != nil {
		fmt.Printf("Cannot generate service descriptor: %v", err)
		fmt.Fprintf(w, "Cannot generate service descriptor: %v", err)
		return
	}
	fmt.Printf("Here's the data:  %s\n", data)
	//fmt.Fprintf(w, "%s", myService)
	json.NewEncoder(w).Encode(myService)
	return
}

func init() {
	appEnv, _ := cfenv.Current()

	myService = cfServiceDiscovery.ServiceDescriptor{
		AppName:     appEnv.Name,
		AppUri:      appEnv.ApplicationURIs[0],
		ServiceName: os.Getenv("SERVICE_NAME"),
		PlanName:    os.Getenv("SERVICE_PLAN"),
	}

	services := appEnv.Services
	if len(services) > 0 {
		//fmt.Printf("redisServiceTag = %v\n", os.Getenv("REDIS_LABEL"))
		//
		//redisServices, err := services.WithLabel(os.Getenv("REDIS_LABEL"))
		fmt.Printf("redisServiceTag = %v\n", myService.ServiceName)
		redisServices, err := services.WithLabel(myService.ServiceName)

		if err != nil || len(redisServices) <= 0 {
			log.Println("No Redis service found!!")
			return
		}

		for credKey, credVal := range redisServices[0].Credentials {
			switch {
			case strings.EqualFold(credKey, "host"):
				redisHost = credVal.(string)

			case strings.EqualFold(credKey, "port"):
				redisPort = fmt.Sprint(credVal)

			case strings.EqualFold(credKey, "password"):
				redisPass = credVal.(string)
			}
		}

		for credKey, credVal := range redisServices[0].Credentials {
			fmt.Printf("credKey: %v, credVal: %v", credKey, credVal)
		}

		redisClient = redis.NewClient(
			&redis.Options{
				Addr:     redisHost + ":" + redisPort,
				Password: redisPass,
				DB:       0,
			})

		// Just care about failures, don't need the actual response
		_, err = redisClient.Ping().Result()

		if err != nil {
			log.Printf("Error pinging Redis: %v", err)
		}
	}
}

func main() {
	fmt.Println("Starting...")
	port := os.Getenv("PORT")
	log.Printf("Listening on port %v", port)
	if len(port) == 0 {
		port = "9000"
	}

	http.HandleFunc("/info", serviceDescriptor)
	http.HandleFunc("/ping", testRedis)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Printf("ListenAndServe: %v", err)
	}
}
