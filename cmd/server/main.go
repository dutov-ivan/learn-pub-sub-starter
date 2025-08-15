package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	fmt.Println("Starting Peril server...")
	gamelogic.PrintServerHelp()
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()
	fmt.Println("Connected to RabbitMQ successfully!")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	key := fmt.Sprintf("%s.*", routing.GameLogSlug)
	ch, q, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, key, pubsub.DurableQueue)
	if err != nil {
		fmt.Println("Couldn't connect to peril_exchange", err)
		return
	}
	fmt.Print(q)
	defer ch.Close()

	for {
		cmds := gamelogic.GetInput()

		to_break_main := false
		for _, c := range cmds {
			switch c {
			case "pause":
				fmt.Println("Sending pause message...")

				err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{
					IsPaused: true,
				})

				if err != nil {
					fmt.Println("Failed to publish a message:", err)
				}
			case "resume":
				fmt.Println("Sending resume message...")
				err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, string(routing.PauseKey), routing.PlayingState{
					IsPaused: false,
				})

				if err != nil {
					fmt.Println("Failed to publish a message:", err)
				}
			case "quit":
				fmt.Println("Exiting")
				to_break_main = true
			default:
				fmt.Printf("Cannot recognize command %s", c)
			}
		}

		if to_break_main {
			break
		}
	}

	<-sigs
	fmt.Println("Shutting down server...")
}
