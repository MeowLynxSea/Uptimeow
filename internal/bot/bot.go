package bot

import (
	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/bot/basic"

	// "encoding/json"
	"errors"
	"log"
)

var (
	client *bot.Client
	player *basic.Player
)

func InitBot(callback func(data string)) {
	client = bot.NewClient()

	player = basic.NewPlayer(client, basic.DefaultSettings)

	err := client.JoinServer("localhost:25565")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Login success")

	var perr bot.PacketHandlerError
	for {
		if err = client.HandleGame(); err == nil {
			panic("HandleGame never return nil")
		}
		if errors.As(err, &perr) {
			log.Print(perr)
		} else {
			log.Fatal(err)
		}
	}
}
