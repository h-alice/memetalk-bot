package main

import (
	"context"
	"log"
	"math/rand/v2"

	irc "github.com/h-alice/irc-client"
)

type MainChatbot struct {
	chatbotName      string // Uses for tracking if bot got mentioned.
	MessageContainer []string

	joinChannels []string
	ircClient    *irc.IrcClient
}

func (cb *MainChatbot) EnqueueMessage(msg string) {
	cb.MessageContainer = append(cb.MessageContainer, msg)
	if len(cb.MessageContainer) > 10 {
		log.Printf("Dequeuing message: %s\n", cb.MessageContainer[0])
		cb.MessageContainer = cb.MessageContainer[1:]
	}
}

// Random sample one message from the container.
func (cb *MainChatbot) MessageSampler() string {
	return cb.MessageContainer[rand.IntN(len(cb.MessageContainer))]
}

func (cb *MainChatbot) mainBotLogic() irc.IrcMessageCallback {

	internal := func(ircc *irc.IrcClient, msg string) error {
		parsed_message, err := irc.ParseIrcMessage(msg)
		if err != nil {
			return err
		}

		// We only care about PRIVMSG messages.
		if parsed_message.Command == "PRIVMSG" {

			// Enqueue the message.
			cb.EnqueueMessage(parsed_message.Message)

			log.Printf("Enqueued message: %s\n", parsed_message.Message)
			// Sample a message.
			sampled_message := cb.MessageSampler()

			log.Printf("Sampled message: %s\n", sampled_message)
		}

		return nil
	}

	return internal
}

func (cb *MainChatbot) Start(ctx context.Context) {

	cb.ircClient.RegisterMessageCallback(cb.mainBotLogic())

	ctx = context.WithoutCancel(ctx)

	client_status := make(chan error)
	go func() {
		client_status <- cb.ircClient.ClientLoop(ctx)
	}()

	// Join channels.
	for _, channel := range cb.joinChannels {
		cb.ircClient.SendMessage(irc.JOIN(channel))
	}

	<-client_status // Wait for client to exit.
	log.Println("Client exited")
}

func NewChatbot(config Config) *MainChatbot {

	ircClient := irc.NewTwitchIrcClient(config.TwitchIrcConfig.Username, config.TwitchIrcConfig.Password)
	return &MainChatbot{
		ircClient:    ircClient,
		joinChannels: config.TwitchIrcConfig.ChannelList,
		chatbotName:  config.TwitchIrcConfig.Username,
	}

}
