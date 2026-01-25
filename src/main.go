package main

import (
	"app/src/infrastructure/config"
	"context"
	"os/signal"
	"syscall"

	// "app/src/infrastructure/postgresql"
	"app/src/infrastructure/rabbitmq"
	"app/src/infrastructure/redis"
	"log"
)

func main() {
	config, _ := config.FetchConfig()

	// err := postgresql.InitPostgresql()
	// if !err.IsNil() {
	// 	log.Fatal(err.JSON())
	// }

	err := rabbitmq.InitRabbitMQ(config, rabbitmq.RequiredModels)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	err = redis.InitRedis(config)
	if !err.IsNil() {
		log.Fatal(err.JSON())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Service started. Waiting for shutdown signal...")
	<-ctx.Done()
	log.Println("Shutdown signal received. Exiting...")
}
