package rabbitmq

import (
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RequiredModels is a starter template for RabbitMQ topology.
// Customize it for your app (names, durability, DLX, routing keys, etc).
//
// Tip: model declarations are idempotent, so calling InitRabbitMQ on every boot is fine.
var RequiredModels = []Model{
	// Exchange template
	ExchangeModel{
		Exchange:   "chatdetective.events",
		Kind:       "topic",
		Durable:    true,
		AutoDelete: false,
		Internal:   false,
		NoWait:     false,
		Args:       amqp.Table{}, // e.g. {"alternate-exchange": "ae.name"}
	},

	QueueModel{
		Queue: "chatdetective.events.queue",
		Durable: true,
		AutoDelete: false,
		Exclusive: false,
		NoWait: false,
		Args: amqp.Table{},
	},

	BindingModel{
		Queue: "chatdetective.events.queue",
		Exchange: "chatdetective.events",
		RoutingKey: fmt.Sprintf("%s.*.*", os.Getenv("POD_ID")),
		NoWait: false,
		Args: amqp.Table{},
	},

	ExchangeModel{
		Exchange: "chatdetective.output.send",
		Kind: "direct",
		Durable: true,
		AutoDelete: false,
		Internal: false,
		NoWait: false,
		Args: amqp.Table{},
	},
}
