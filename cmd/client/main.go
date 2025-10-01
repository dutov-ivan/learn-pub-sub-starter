package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	"github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, war_ch *amqp091.Channel, war_key string) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(am gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(am)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			err := pubsub.PublishJSON(war_ch, routing.ExchangePerilTopic, war_key, gamelogic.RecognitionOfWar{
				Attacker: am.Player,
				Defender: gs.Player,
			})
			if err != nil {
				return pubsub.NackRequeue
			}

			return pubsub.Ack

		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			fmt.Printf("unexpected gamelogic.MoveOutcome: %#v", outcome)
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)
		var log_message string

		switch outcome {
		case gamelogic.WarOutcomeDraw:
			log_message = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			return pubsub.Ack
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeOpponentWon:
			log_message = fmt.Sprint("%s won a war against %s", winner, loser)
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			log_message = fmt.Sprint("%s won a war against %s", winner, loser)
			return pubsub.Ack
		default:
			(fmt.Printf("unexpected gamelogic.WarOutcome: %#v", outcome))
			return pubsub.NackDiscard
		}
	}
}

func main() {
	fmt.Println("Starting Peril client...")

	conn, err := amqp091.Dial("amqp://guest:guest@localhost:5672")
	if err != nil {
		fmt.Println("Failed to connect to RabbitMQ", err)
		return
	}
	defer conn.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println("Failed to get username", err)
		return
	}

	gameState := gamelogic.NewGameState(username)

	war_key := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, username)
	war_ch, war_q, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, war_key, pubsub.DurableQueue)

	if err != nil {
		fmt.Println("Failed to declare and/or bind a war queue", err)
		return
	}
	defer war_ch.Close()

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, war_q.Name, "war.*", pubsub.DurableQueue, handlerWar(gameState))
	if err != nil {
		fmt.Println("Failed to setup War consumer:", err)
		return
	}

	pauseQueue := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	pause_ch, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, pauseQueue, routing.PauseKey, pubsub.TransientQueue)
	if err != nil {
		fmt.Println("Failed to declare and/or bind a pause queue", err)
		return
	}
	defer pause_ch.Close()

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, pauseQueue, routing.PauseKey, pubsub.TransientQueue, handlerPause(gameState))
	if err != nil {
		fmt.Println("Failed to setup Pause consumer:", err)
		return
	}

	army_key := fmt.Sprintf("%s.*", routing.ArmyMovesPrefix)
	army_queue := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username)

	err = pubsub.SubscribeJSON(conn, string(routing.ExchangePerilTopic), army_queue, army_key, pubsub.TransientQueue, handlerMove(gameState, war_ch, war_key))
	if err != nil {
		fmt.Println("Failed to setup army subscriber", err)
		return
	}

	for {
		cmds := gamelogic.GetInput()

		to_break_main := false
		c := cmds[0]
		switch c {
		case "spawn":
			err = gameState.CommandSpawn(cmds)
			if err != nil {
				fmt.Println(err)
				continue
			}
		case "move":
			move, err := gameState.CommandMove(cmds)
			if err != nil {
				fmt.Println(err)
				continue
			}

			ch, _, err := pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, army_queue, army_queue, pubsub.TransientQueue)
			if err != nil {
				fmt.Println(err)
				continue
			}
			defer ch.Close()
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, army_queue, move)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Println("Move published successfully.")
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming is not allowed yet!")

		case "quit":
			gamelogic.PrintQuit()
			to_break_main = true
		default:
			fmt.Printf("Cannot recognize command '%s'\n", c)
		}

		if to_break_main {
			break
		}
	}
}
