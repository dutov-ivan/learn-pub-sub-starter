package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, msg T) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

type simpleQueueType int

const (
	DurableQueue simpleQueueType = iota
	TransientQueue
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	isDurable := queueType == DurableQueue
	isAutoDelete := queueType == TransientQueue
	isExclusive := queueType == TransientQueue

	q, err := ch.QueueDeclare(queueName, isDurable, isAutoDelete, isExclusive, false, amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	})
	if err != nil {
		return ch, amqp.Queue{}, err
	}

	err = ch.QueueBind(q.Name, key, exchange, false, nil)
	if err != nil {
		return ch, q, err
	}

	return ch, q, nil
}

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType simpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	ch, q, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	delivery_ch, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer ch.Close()

		for msg := range delivery_ch {
			var val T
			err = json.Unmarshal(msg.Body, &val)
			if err != nil {
				fmt.Println(err)
				return
			}

			res := handler(val)
			switch res {
			case Ack:
				msg.Ack(false)
				fmt.Println("Acknowledged")

			case NackRequeue:
				msg.Nack(false, true)
				fmt.Println("Not Acknowledged, requeue")

			case NackDiscard:
				msg.Nack(false, false)
				fmt.Println("Not Acknowledged, discard")

			}
		}
	}()

	return nil
}
