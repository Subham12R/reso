package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/subham12r/reso/internal/api"
	"github.com/subham12r/reso/internal/api/handlers"
	"github.com/subham12r/reso/internal/config"
	"github.com/subham12r/reso/internal/queue"
	"github.com/subham12r/reso/internal/redisclient"
	"github.com/subham12r/reso/internal/rooms"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	configuration, err := config.Load()
	if err != nil {
		return err
	}

	redisClient, err := redisclient.New(configuration.RedisURL)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return err
	}

	roomService := rooms.NewRoomServiceWithStore(rooms.NewRedisStore(redisClient))

	server := &http.Server{
		Addr: ":8080",
		Handler: api.NewRouterWithOptions(
			roomService,
			queue.NewService(redisClient),
			handlers.MediaConfig{URL: configuration.LiveKitURL, APIKey: configuration.LiveKitAPIKey, Secret: configuration.LiveKitSecret},
			api.RouterOptions{
				Redis:             redisClient,
				LiveKitURL:        configuration.LiveKitURL,
				Logger:            slog.Default(),
				TrustProxyHeaders: configuration.TrustProxyHeaders,
			},
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.ListenAndServe()
	}()

	fmt.Println("Server is running on :8080")
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
