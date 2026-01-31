package application

import (
	e "app/pkg/errors"
	"app/src/infrastructure/config"
	"app/src/infrastructure/rabbitmq"
	"app/src/infrastructure/redis"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	tele "gopkg.in/telebot.v4"

	amqp "github.com/rabbitmq/amqp091-go"
)

func initRabbitmqQueue(cfg *config.Config) (<-chan amqp.Delivery, *amqp.Channel, *e.ErrorInfo) {
	rabbitmqChannel, err := rabbitmq.NewRabbitmqChannel(cfg)
	if !err.IsNil() {
		return nil, nil, err
	}

	consumer, unwrappedError := rabbitmqChannel.Consume(
		"chatdetective.events.queue",
		"main-consumer",
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

var (
	gorutineEndpoints = make(map[string]chan amqp.Delivery, 1000)
	errors = make(chan *e.ErrorInfo, 1000)
)

type WorkerArgs struct {
	wg *sync.WaitGroup;
	ctx context.Context;
	updatesChan chan amqp.Delivery;
	cfg *config.Config
	sessionId string
}

func ListenToRabbitmq(cfg *config.Config, ctx context.Context, wg *sync.WaitGroup) *e.ErrorInfo {
	consumer, rabbitmqChannel, err := initRabbitmqQueue(cfg)
	if !err.IsNil() {
		return err
	}
	defer rabbitmqChannel.Close()

	for {
		select {
		case <-ctx.Done():
			rabbitmqChannel.Cancel("main-consumer", false)
			return e.Nil()
		case delivery, ok := <-consumer:
			if !ok {
				// consumer channel closed
				return e.FromError(nil, "RabbitMQ consumer channel closed").WithSeverity(e.Critical)
			}
			initNewHandlerOrPushToExisting(delivery, ctx, wg, cfg)
		}
	}
}

func initNewHandlerOrPushToExisting(delivery amqp.Delivery, ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) {
	sessionId := strings.Split(delivery.RoutingKey, ".")[2]

	if _, ok := gorutineEndpoints[sessionId]; ok {
		gorutineEndpoints[sessionId] <- delivery
	} else {
		destChan := initHandler(ctx, wg, cfg, sessionId)
		destChan <- delivery
		gorutineEndpoints[sessionId] = destChan
	}

	// close(delivery.Ack(false))
}

func initHandler(ctx context.Context, wg *sync.WaitGroup, cfg *config.Config, sessionId string) chan amqp.Delivery {
	updatesChan := make(chan amqp.Delivery, 1000)

	wg.Add(1)
	go handle(WorkerArgs{
		wg: wg,
		ctx: ctx,
		updatesChan: updatesChan,
		cfg: cfg,
		sessionId: sessionId,
	})

	return updatesChan
}

func handle(args WorkerArgs) {
	args.wg.Add(1)
	defer args.wg.Done()

	for {
		select {
		case <-args.ctx.Done():
			delete(gorutineEndpoints, args.sessionId)
			close(args.updatesChan)
			return
		case <-time.After(args.cfg.RuntimeConfig.HandlerLiveDuration):
			delete(gorutineEndpoints, args.sessionId)
			// TODO: Добавить подтверждение завершения обработки всех текущих апдейтов
			close(args.updatesChan)
			return
		case delivery := <-args.updatesChan:
			updateBytes := delivery.Body
			var update tele.Update
			unwrappedError := json.Unmarshal(updateBytes, &update)
			if unwrappedError != nil {
				errors <- e.FromError(unwrappedError, "failed to unmarshal update").WithSeverity(e.Critical)
				continue
			}
			router.Dispatch(update)
		}
	}
}


var router Router = Router{
	ErrorChannel: errors,
	Endpoints: []Endpoint{
		{
			handler: func(update tele.Update, timeout time.Duration) (handlerResponse, *e.ErrorInfo) {
				return handlerResponse{
					Method: "send_message",
					SendData: map[string]any{
						"text": "Hello, world!",
					},
					SenderBot: "@main",
				}, e.Nil()
			},
			filter: Or(Command([]string{"test", "start"}), TextCommand("test text command")),
			timeout: time.Second * 10,
			Name: "test",
		},
	},
}

func changePodList(script string, cfg *config.Config) *e.ErrorInfo {
	redisScript := redigo.NewScript(1, script)

	redisConnection, err := redis.NewRedisConnection(cfg)
	if !err.IsNil() {
		return err
	}
	defer redisConnection.Close()

	key := fmt.Sprintf("pods:handlers:%s:load", cfg.RuntimeConfig.PodType)
	_, unwrappedError := redisScript.Do(redisConnection, key, cfg.RuntimeConfig.PodID)
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
