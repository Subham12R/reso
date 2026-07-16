package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/subham12r/reso/internal/api"
	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/config"
	"github.com/subham12r/reso/internal/queue"
	"github.com/subham12r/reso/internal/redisclient"
	"github.com/subham12r/reso/internal/rooms"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	redisClient, err := redisclient.New(config.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal(err)
	}

	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(redisClient))

	server := &http.Server{
		Addr:    ":8080",
		Handler: api.NewRouter(roomService, queue.NewService(redisClient), handlers.MediaConfig{URL: config.LiveKitURL, APIKey: config.LiveKitAPIKey, Secret: config.LiveKitSecret}),
	}
	fmt.Println("Server is running on :8080")
	log.Fatal(server.ListenAndServe())
}
