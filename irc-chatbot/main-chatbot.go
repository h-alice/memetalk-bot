package main

import (
	"context"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	irc "github.com/h-alice/irc-client"
)

type BotMessage struct {
	Username string
	Message  string
	Channel  string
}

func (bm BotMessage) String() string {
	return "[" + bm.Channel + "] " + bm.Username + ": " + bm.Message
}

type MainChatbot struct {
	chatbotName            string // Uses for tracking if bot got mentioned.
	chatbotDisplayName     string
	MessageSampleContainer []BotMessage
	MessageReplyQueue      chan BotMessage

	joinChannels []string
	ircClient    *irc.IrcClient

	minReplyDelaySeconds          int
	maxReplyDelaySeconds          int
	minReplyChatStallDelaySeconds int
	maxReplyChatStallDelaySeconds int

	// Non-Configurable local fields.
	lastMessageTime time.Time
}

func (cb *MainChatbot) EnqueueMessage(msg irc.IrcMessage) {

	bm := BotMessage{
		Username: msg.Prefix.Username,
		Message:  msg.Message,
		Channel:  msg.Params[0],
	}

	cb.MessageSampleContainer = append(cb.MessageSampleContainer, bm)
	if len(cb.MessageSampleContainer) > 10 {
		log.Printf("<MSGQUEUE> Dequeuing message: %s\n", cb.MessageSampleContainer[0])
		cb.MessageSampleContainer = cb.MessageSampleContainer[1:]
	}
}

func (cb *MainChatbot) EnqueueMentionedMessage(msg irc.IrcMessage) {
	bm := BotMessage{
		Username: msg.Prefix.Username,
		Message:  msg.Message,
		Channel:  msg.Params[0],
	}

	// Directly enqueue the message to reply queue.
	cb.enqueueMessageToReply(bm)
}

func (cb *MainChatbot) enqueueMessageToReply(msg BotMessage) {

	// NOTE: Enqueue without blocking.
	select {

	case cb.MessageReplyQueue <- msg:
		log.Printf("<REPLY QUEUE> Enqueued message to reply: %s\n", msg)

	default:
		log.Printf("<REPLY QUEUE> Message reply queue is full. Dropping message: %s\n", msg)

	}
}

// Random sample one message from the container.
func (cb *MainChatbot) MessageSampler() BotMessage {
	return cb.MessageSampleContainer[rand.IntN(len(cb.MessageSampleContainer))]
}

func (cb *MainChatbot) messageSamplerLoop(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return

		default:

			stall_delay_delta := cb.maxReplyChatStallDelaySeconds - cb.minReplyChatStallDelaySeconds

			if stall_delay_delta == 0 {
				stall_delay_delta = 1 // Avoid invalid sampling interval.
			}

			if time.Since(cb.lastMessageTime) < time.Duration(
				rand.IntN(stall_delay_delta)+cb.minReplyChatStallDelaySeconds)*time.Second {
				continue // Stop sampling if chat is stalled.
			}

			if len(cb.MessageSampleContainer) == 0 {
				continue
			}

			// Randomly sample a message from the container.
			sampled_message := cb.MessageSampler()
			// Enqueue the message to reply.
			cb.enqueueMessageToReply(sampled_message)

			// Apply delay before replying.
			delay_seconds := rand.IntN(cb.maxReplyDelaySeconds-cb.minReplyDelaySeconds) + cb.minReplyDelaySeconds
			time.Sleep(time.Duration(delay_seconds) * time.Second)
		}
	}
}

func (cb *MainChatbot) botReplyLoop(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-cb.MessageReplyQueue:
			// Just print to stdout for now.
			// Sample a message.
			log.Printf("<REPLY> Sampled message: %s\n", msg)
			log.Println(irc.PRIVMSG(msg.Channel, ""+msg.Message))
			cb.ircClient.SendMessage(irc.PRIVMSG(msg.Channel, msg.Message))

			// Safe Guard: Delay at least 1 second before replying.
			time.Sleep(1 * time.Second)
		}
	}
}

func (cb *MainChatbot) mainBotLogic() irc.IrcMessageCallback {

	internal := func(ircc *irc.IrcClient, msg string) error {
		parsed_message, err := irc.ParseIrcMessage(msg)
		if err != nil {
			return err
		}

		// We only care about PRIVMSG messages.
		if parsed_message.Command == "PRIVMSG" {

			// Update last message time.
			cb.lastMessageTime = time.Now()

			// Handle direct mentions.
			name_tag := "@" + cb.chatbotDisplayName

			// Check if name tag is in the message.
			if strings.Contains(parsed_message.Message, name_tag) {
				log.Printf("<CALLBACK> Got mentioned: %s\n", parsed_message.Message)

				// Remove the metion tag from message.
				parsed_message.Message = strings.ReplaceAll(parsed_message.Message, name_tag, "")

				// Directly enqueue the message to reply queue.
				cb.enqueueMessageToReply(BotMessage{
					Username: parsed_message.Prefix.Username,
					Message:  parsed_message.Message,
					Channel:  parsed_message.Params[0],
				})
			}

			// Enqueue the message.
			cb.EnqueueMessage(parsed_message)

			log.Printf("<CALLBACK> Enqueued message: %s\n", parsed_message.Message)

		}

		return nil
	}

	return internal
}

func (cb *MainChatbot) Start(ctx context.Context) {

	cb.ircClient.RegisterMessageCallback(cb.mainBotLogic())

	ctx, cancel := context.WithCancel(ctx)

	// Start message sampler loop.
	cb.MessageReplyQueue = make(chan BotMessage, 100)
	go cb.messageSamplerLoop(ctx)

	// Start bot reply loop.
	go cb.botReplyLoop(ctx)

	// Start IRC client.
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

	cancel() // Cleanup
}

func NewChatbot(config Config) *MainChatbot {

	ircClient := irc.NewTwitchIrcClient(config.TwitchIrcConfig.Username, config.TwitchIrcConfig.Password)
	return &MainChatbot{
		ircClient:          ircClient,
		joinChannels:       config.TwitchIrcConfig.ChannelList,
		chatbotName:        config.TwitchIrcConfig.Username,
		chatbotDisplayName: config.TwitchIrcConfig.DisplayName,

		minReplyDelaySeconds:          config.ChatbotSetting.ReplySetting.ReplyMinDelaySeconds,
		maxReplyDelaySeconds:          config.ChatbotSetting.ReplySetting.ReplyMaxDelaySeconds,
		minReplyChatStallDelaySeconds: config.ChatbotSetting.ReplySetting.MinReplyChatStallDelaySeconds,
		maxReplyChatStallDelaySeconds: config.ChatbotSetting.ReplySetting.MaxReplyChatStallDelaySeconds,
	}

}
