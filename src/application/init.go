package application

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	businessconnection "github.com/ChatDetectiveORG/command-handler/src/application/endpoints/businessConnection"
	checkconnection "github.com/ChatDetectiveORG/command-handler/src/application/endpoints/checkConnection"
	deletedata "github.com/ChatDetectiveORG/command-handler/src/application/endpoints/deleteData"
	exportchat "github.com/ChatDetectiveORG/command-handler/src/application/endpoints/exportChat"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/help"
	howencryption "github.com/ChatDetectiveORG/command-handler/src/application/endpoints/howEncryption"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/installation"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/mirror"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/referral"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/settings"
	"github.com/ChatDetectiveORG/command-handler/src/application/endpoints/start"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/config"
	"github.com/ChatDetectiveORG/command-handler/src/infrastructure/rabbitmq"

	e "github.com/ChatDetectiveORG/shared/errors"
	h "github.com/ChatDetectiveORG/shared/handlers"
	"github.com/ChatDetectiveORG/shared/amqputil"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	errors          = make(chan *e.ErrorInfo, 1000)
	rabbitmqChannel *amqp.Channel
)

const shardCount = 64

func initRabbitmqQueue(cfg *config.Config) (<-chan amqp.Delivery, []string, *amqp.Channel, *e.ErrorInfo) {
	rabbitmqChannel, err := rabbitmq.NewRabbitmqChannel(cfg)
	if !err.IsNil() {
		return nil, nil, nil, err
	}

	_ = rabbitmqChannel.Qos(50, 0, false)

	merged := make(chan amqp.Delivery, 1000)
	consumerTags := make([]string, 0, shardCount)

	var forwardWg sync.WaitGroup
	forwardWg.Add(shardCount)

	podID := os.Getenv("POD_ID")
	if podID == "" {
		podID = "unknown"
	}

	for i := 0; i < shardCount; i++ {
		q := fmtShardQueue(i)
		tag := fmt.Sprintf("events-%s-%s", podID, q)
		consumerTags = append(consumerTags, tag)

		consumer, unwrappedError := rabbitmqChannel.Consume(
			q,
			tag,
			false,
			false,
			false,
			false,
			amqp.Table{},
		)
		if unwrappedError != nil {
			_ = rabbitmqChannel.Close()
			return nil, nil, nil, e.FromError(unwrappedError, "failed to init consumer").WithSeverity(e.Critical).WithData(map[string]any{"queue": q})
		}

		go func(c <-chan amqp.Delivery) {
			defer forwardWg.Done()
			for d := range c {
				merged <- d
			}
		}(consumer)
	}

	go func() {
		forwardWg.Wait()
		close(merged)
	}()

	return merged, consumerTags, rabbitmqChannel, e.Nil()
}

func ListenToRabbitmq(cfg *config.Config, ctx context.Context, wg *sync.WaitGroup) *e.ErrorInfo {
	router.ReplicaCount = cfg.RuntimeConfig.NumRoutingGorutines
	if router.ReplicaCount <= 0 {
		router.ReplicaCount = shardCount
	}
	podID := cfg.RuntimeConfig.PodID
	if podID == "" {
		podID = "unknown"
	}
	router.InitSharding(podID, wg, ctx)

	go hanleError(errors, ctx, wg)

	wg.Add(1)
	go func() {
		defer wg.Done()
		amqputil.RunConsumerLoop(ctx, amqputil.ConsumerConfig{
			Dial: func() (*amqputil.ConsumerSession, error) {
				deliveries, tags, ch, dialErr := initRabbitmqQueue(cfg)
				if !dialErr.IsNil() {
					return nil, dialErr
				}
				return &amqputil.ConsumerSession{
					Deliveries: deliveries,
					Channel:    ch,
					Cleanup: func() {
						for _, tag := range tags {
							_ = ch.Cancel(tag, false)
						}
						_ = ch.Close()
					},
				}, nil
			},
			OnConnect: func(session *amqputil.ConsumerSession) {
				if connectErr := router.ConnectRabbitmqSession(session.Channel, openRabbitmqChannel(cfg), wg, podID, ctx); !connectErr.IsNil() {
					log.Printf("command-handler: outgoing session setup failed: %s", connectErr.JSON())
				}
			},
			OnDelivery: func(delivery amqp.Delivery) {
				log.Printf("trace=%s received rk=%s", delivery.CorrelationId, delivery.RoutingKey)
				if routeErr := router.HandleUpdate(delivery); !routeErr.IsNil() {
					errors <- routeErr.WithData(map[string]any{"rk": delivery.RoutingKey}).WithSeverity(e.Critical)
					if nackErr := delivery.Nack(false, false); nackErr != nil {
						errors <- e.FromError(nackErr, "failed to nack delivery").WithSeverity(e.Critical)
					}
					return
				}
				if ackErr := delivery.Ack(false); ackErr != nil {
					errors <- e.FromError(ackErr, "Ошибка подтверждения получения")
				}
			},
		})
	}()

	return e.Nil()
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

func fmtShardQueue(i int) string {
	return fmt.Sprintf("%s.q%02d", config.PodType, i)
}

func openRabbitmqChannel(cfg *config.Config) func() (*amqp.Channel, error) {
	return func() (*amqp.Channel, error) {
		ch, err := rabbitmq.NewRabbitmqChannel(cfg)
		if !err.IsNil() {
			return nil, err
		}
		return ch, nil
	}
}

var router h.Router = h.Router{
	ErrorChannel:    errors,
	RabbitmqChannel: rabbitmqChannel,
	Endpoints: []h.Endpoint{
		// /start command, legal consent gate and show-contacts callback
		start.NewStartEndpoint(),
		start.NewLegalConsentEndpoint(),
		start.NewShowContactsEndpoint(),

		// Installation guide
		installation.NewInstallationEndpoint(),

		// Settings page + toggle callbacks
		settings.NewSettingsEndpoint(),
		settings.NewToggleDeletedEndpoint(),
		settings.NewToggleEditedEndpoint(),
		settings.NewToggleSelfMediaEndpoint(),
		settings.NewToggleExtExportEndpoint(),

		// Help command
		help.NewHelpEndpoint(),

		// Business connection connect/disconnect
		businessconnection.NewBusinessConnectionEndpoint(),

		// Check connection
		checkconnection.NewCheckConnectionEndpoint(),

		// Mirror creation
		mirror.NewCreateMirrorEndpoint(),
		mirror.NewMirrorTokenEndpoint(),
		mirror.NewMirrorListEndpoint(),
		mirror.NewMirrorInfoEndpoint(),
		mirror.NewMirrorDeleteEndpoint(),

		// Referral program + all sub-callbacks
		referral.NewReferralEndpoint(),
		referral.NewBonusSelectEndpoint(),
		referral.NewBonusDetailsEndpoint(),
		referral.NewBonusBackEndpoint(),
		referral.NewBonusMoneyEndpoint(),
		referral.NewBonusLevelsEndpoint(),
		referral.NewWhatLevelsEndpoint(),
		referral.NewUpgradeLevelEndpoint(),
		referral.NewUpgradeLevelCommandEndpoint(),
		referral.NewLevelCommandEndpoint(),

		// How encryption works
		howencryption.NewHowEncryptionEndpoint(),

		// Delete data command + confirm/cancel callbacks
		deletedata.NewDeleteDataEndpoint(),
		deletedata.NewDeleteConfirmEndpoint(),
		deletedata.NewDeleteCancelEndpoint(),

		// Chat export: list → preview → invoice. Actual export pipeline lives in chat-export-service.
		exportchat.NewSelectChatEndpoint(),
		exportchat.NewViewChatEndpoint(),
		exportchat.NewRestoreChatEndpoint(),
	},
}
