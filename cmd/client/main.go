package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril client...")

	conn, err := amqp091.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ", err)
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println("Failed to get username", err)
		return
	}

	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	ch, q, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TransientQueue)
	if err != nil {
		fmt.Println("Failed to declare and/or bind a queue", ch, q, err)
		return
	}

	<-sigs
	gamelogic.PrintQuit()

}
