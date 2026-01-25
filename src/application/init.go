package application

import (
	e "app/pkg/errors"
	"app/src/infrastructure/config"
	"app/src/infrastructure/rabbitmq"
	"app/src/infrastructure/redis"
	"fmt"

	redigo "github.com/gomodule/redigo/redis"

	amqp "github.com/rabbitmq/amqp091-go"
)

func initRabbitmqQueue(cfg *config.Config) (<-chan amqp.Delivery, *amqp.Channel, *e.ErrorInfo) {
	rabbitmqChannel, err := rabbitmq.NewRabbitmqChannel(cfg)
	if !err.IsNil() {
		return nil, nil, err
	}

	queue, unwrappedError := rabbitmqChannel.QueueDeclare(
		"chatdetective.events.queue",
		true,
		false,
		false,
		false,
		amqp.Table{},
	)
	if unwrappedError != nil {
		return nil, nil, e.FromError(unwrappedError, "Failed to init queue chatdetective.events.queue").WithSeverity(e.Critical)
	}

	unwrappedError = rabbitmqChannel.QueueBind(
		"chatdetective.events.queue",
		fmt.Sprintf("%s.*.*", cfg.RuntimeConfig.PodID),
		"chatdetective.events",
		false,
		amqp.Table{},
	)
	if unwrappedError != nil {
		return nil, nil, e.FromError(unwrappedError, "Failed to init bind queue chatdetective.events.queue").WithSeverity(e.Critical)
	}

	consumer, unwrappedError := rabbitmqChannel.Consume(
		queue.Name,
		"",
		true, // Автоматически подтверждать поулчение
		false,
		false,
		false,
		amqp.Table{},
	)
	if unwrappedError != nil {
		return nil, nil, e.FromError(unwrappedError, "Failed to init consumer for queue chatdetective.events.queue").WithSeverity(e.Critical)
	}

	return consumer, rabbitmqChannel, e.Nil()
}

// func ListenToRabbitmq(cfg *config.Config) *e.ErrorInfo {
// 	consumer, rabbitmqChannel, err := initRabbitmqQueue(cfg)
// 	if !err.IsNil() {
// 		return err
// 	}

// }

func changePodList(script string, cfg *config.Config) *e.ErrorInfo {
	redisScript := redigo.NewScript(1, script)

	redisConnection, err := redis.NewRedisConnection(cfg)
	if !err.IsNil() {
		return err
	}
	defer redisConnection.Close()

	key := fmt.Sprintf("pods:handlers:%s:load", cfg.RuntimeConfig.PodType)
	_, unwrappedError := redisScript.Do(redisConnection, key)
	if unwrappedError != nil {
		return e.FromError(unwrappedError, "Failed to execute script").WithSeverity(e.Critical)
	}

	return e.Nil()
}

func MakeAstatement(cfg *config.Config) *e.ErrorInfo {
	err := changePodList(`
		local key = KEYS[1]
		local pod_id = ARGV[1]

		return redis.call('ZADD', key, 0, pod_id)
	`, cfg)
	
	if !err.IsNil() {
		return err.PushStack().WithData(map[string]any{"operation": "adding myself to pods zset"})
	}

	return e.Nil()
}

func DeleteMyselfFromRedis(cfg *config.Config) *e.ErrorInfo {
	err := changePodList(`
		local key = KEYS[1]
		local pod_id = ARGV[1]

		return redis.call('ZREM', key, pod_id)
	`, cfg)

	if !err.IsNil() {
		return err.PushStack().WithData(map[string]any{"operation": "removing myself from pods zset"})
	}

	return e.Nil()
}
