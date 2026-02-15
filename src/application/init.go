package application

import (
	e "app/pkg/errors"
	"app/src/infrastructure/config"
	"app/src/infrastructure/rabbitmq"
	"app/src/infrastructure/redis"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	redigo "github.com/gomodule/redigo/redis"
	tele "gopkg.in/telebot.v4"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	gorutineEndpoints = make(map[string]chan amqp.Delivery, 1000)
	handlerTerminated = make(chan string, 1000)
	errors            = make(chan *e.ErrorInfo, 1000)
	rabbitmqChannel   *amqp.Channel
	gcfg              *config.Config
)

func initRabbitmqQueue(cfg *config.Config) (<-chan amqp.Delivery, *amqp.Channel, *e.ErrorInfo) {
	rabbitmqChannel, err := rabbitmq.NewRabbitmqChannel(cfg)
	if !err.IsNil() {
		return nil, nil, err
	}

	consumer, unwrappedError := rabbitmqChannel.Consume(
		"chatdetective.events.queue",
		"main-consumer",
		false, // Автоматически подтверждать поулчение
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

type WorkerArgs struct {
	wg          *sync.WaitGroup
	ctx         context.Context
	updatesChan chan amqp.Delivery
	cfg         *config.Config
	sessionId   string
}

func ListenToRabbitmq(cfg *config.Config, ctx context.Context, wg *sync.WaitGroup) *e.ErrorInfo {
	gcfg = cfg // Чёрт, что я творю.........
	var consumer <-chan amqp.Delivery
	var err *e.ErrorInfo
	consumer, rabbitmqChannel, err = initRabbitmqQueue(cfg)
	if !err.IsNil() {
		return err
	}
	// router is a package-level singleton and was initialized before rabbitmqChannel was set.
	// Update it to use the live channel to avoid nil deref in PublishWithContext.
	router.RabbitmqChannel = rabbitmqChannel
	defer rabbitmqChannel.Close()

	go hanleError(errors, ctx, wg)

	for {
		select {
		case <-ctx.Done():
			rabbitmqChannel.Cancel("main-consumer", false)
			for sessionId, ch := range gorutineEndpoints {
				delete(gorutineEndpoints, sessionId)
				close(ch)
			}
			return e.Nil()
		case sessionId := <-handlerTerminated:
			if ch, ok := gorutineEndpoints[sessionId]; ok {
				delete(gorutineEndpoints, sessionId)
				close(ch)
			}
		case delivery, ok := <-consumer:
			if !ok {
				// consumer channel closed
				// ToDo: Не подтверждать доставку
				return e.FromError(nil, "RabbitMQ consumer channel closed").WithSeverity(e.Critical)
			}
			initNewHandlerOrPushToExisting(delivery, ctx, wg, cfg)
		}
	}
}

func hanleError(src chan (*e.ErrorInfo), context context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()

	for {
		select {
		case <-context.Done():
			return
		case err := <-src:
			log.Println(err.JSON())
		}
	}
}

func initNewHandlerOrPushToExisting(delivery amqp.Delivery, ctx context.Context, wg *sync.WaitGroup, cfg *config.Config) {
	keyParams := strings.Split(delivery.RoutingKey, ".")
	if len(keyParams) != 3 {
		errors <- e.NewError("Inavlid routing key!", "")
		return
	}

	sessionId := keyParams[2]

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
		wg:          wg,
		ctx:         ctx,
		updatesChan: updatesChan,
		cfg:         cfg,
		sessionId:   sessionId,
	})

	return updatesChan
}

func handle(args WorkerArgs) {
	defer args.wg.Done()

	timer := time.NewTimer(args.cfg.RuntimeConfig.HandlerLiveDuration)
	defer timer.Stop()

	for {
		select {
		case <-args.ctx.Done():
			// On shutdown the main loop may already be gone; don't block here.
			select {
			case handlerTerminated <- args.sessionId:
			default:
			}
			return
		case <-timer.C:
			handlerTerminated <- args.sessionId
			// TODO: Добавить подтверждение завершения обработки всех текущих апдейтов
			return
		case delivery, ok := <-args.updatesChan:
			if !ok {
				return
			}
			log.Printf("trace=%s received rk=%s", delivery.CorrelationId, delivery.RoutingKey)
			updateBytes := delivery.Body
			var update tele.Update
			unwrappedError := json.Unmarshal(updateBytes, &update)
			if unwrappedError != nil {
				errors <- e.FromError(unwrappedError, "failed to unmarshal update").WithSeverity(e.Critical)
				continue
			}
			router.Dispatch(update)
			// TODO: Сделать подтверждение зависимым от исхода обработки в Dispatch
			unwrappedError = delivery.Ack(false)

			if unwrappedError != nil {
				errors <- e.FromError(unwrappedError, "Ошибка подтверждения получения")
			}

			err := DecreacePodLoad(gcfg)
			if !err.IsNil() {
				errors <- err.PushStack()
			}

			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(args.cfg.RuntimeConfig.HandlerLiveDuration)
		}
	}
}

var router Router = Router{
	ErrorChannel:    errors,
	RabbitmqChannel: rabbitmqChannel,
	Endpoints: []Endpoint{
		{
			handler: func(update tele.Update, timeout time.Duration) (handlerResponse, *e.ErrorInfo) {
				return handlerResponse{
					Method: "text",
					SendData: map[string]any{
						"text": "Hello, world!",
						"chat_id": update.Message.Chat.ID,
					},
					SenderBot: "@main",
				}, e.Nil()
			},
			filter:  Or(Command([]string{"test", "start"}), TextCommand("test text command")),
			timeout: time.Second * 10,
			Name:    "test",
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

func DecreacePodLoad(cfg *config.Config) *e.ErrorInfo {
	err := changePodList(`
		local pod_id = ARGV[1]

		local exists = redis.call("ZSCORE", KEYS[1], pod_id)

		if exists then
			redis.call("ZINCRBY", KEYS[1], -1, pod_id)
			return pod_id
		end

		return nil
	`, cfg)

	if !err.IsNil() {
		return err.PushStack().WithData(map[string]any{"operation": "decreacing my load"})
	}

	return e.Nil()
}
