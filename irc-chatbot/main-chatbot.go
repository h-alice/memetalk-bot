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
	chatbotName            string
	chatbotDisplayName     string // Uses for tracking if bot got mentioned.
	MessageSampleContainer []BotMessage
	MessageReplyQueue      chan BotMessage

	joinChannels []string
	ircClient    *irc.IrcClient

	minReplyDelaySeconds          int
	maxReplyDelaySeconds          int
	minReplyChatStallDelaySeconds int
	maxReplyChatStallDelaySeconds int

	// LLM prompt crafter.
	llmPromptTemplate      string
	llmBosToken            string
	llmAddGenerationPrompt bool

	chatHistoryLookupLimit int

	// Local fields.
	lastMessageTime time.Time
	lastSampleTime  time.Time

	chatHistory []PromptMessage
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

// # Method: messageSamplerLoop
//
// Randomly sample messages from the container and reply.
func (cb *MainChatbot) messageSamplerLoop(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return

		default:

			// Check tall condition.
			stall_condition := cb.lastSampleTime.After(cb.lastMessageTime)

			// Calculate random delay time = random_sample(time_delta) + min_delay
			stall_delay_delta := cb.maxReplyChatStallDelaySeconds - cb.minReplyChatStallDelaySeconds // Time delta.

			// Randomly sample delay time.
			sampled_delay_time := time.Duration(rand.IntN(stall_delay_delta)+cb.minReplyChatStallDelaySeconds) * time.Second // Random delay time.
			if stall_delay_delta == 0 {
				stall_delay_delta = 1 // Avoid invalid sampling interval.
			}

			if stall_condition {
				if time.Since(cb.lastSampleTime) < sampled_delay_time {
					continue // Stop sampling if chat is stalled.
				}
			}

			if len(cb.MessageSampleContainer) == 0 {
				continue
			}

			// Randomly sample a message from the container.
			sampled_message := cb.MessageSampler()
			// Enqueue the message to reply.
			cb.enqueueMessageToReply(sampled_message)

			// Debug.
			if stall_condition {
				log.Printf("<SAMPLER> Message send during stall: %s\n", sampled_message)
			}

			// Apply delay before replying.
			delay_seconds := rand.IntN(cb.maxReplyDelaySeconds-cb.minReplyDelaySeconds) + cb.minReplyDelaySeconds

			cb.lastSampleTime = time.Now() // Update last sample time.

			time.Sleep(time.Duration(delay_seconds) * time.Second)
		}
	}
}

func (cb *MainChatbot) appendChatHistory(msg_user PromptMessage, msg_bot PromptMessage) {
	cb.chatHistory = append(cb.chatHistory, msg_user, msg_bot)
}

func (cb *MainChatbot) replyPromptCrafter(messages string) (string, error) {
	// Get last lookup limit messages.

	var full_message_stack []PromptMessage
	// Note that every (user, bot) message pair ia one history stack.
	// So that we need to get last lookup_limit * 2 messages.
	lookup_limit := cb.chatHistoryLookupLimit * 2

	lookup_index := len(cb.chatHistory) - lookup_limit

	if lookup_index <= 0 || lookup_index >= len(cb.chatHistory) {
		// Do nothing.
	} else {
		full_message_stack = cb.chatHistory[lookup_index:]
	}

	// Append user messages.
	full_message_stack = append(full_message_stack, PromptMessage{Role: "user", Content: messages})

	// Render prompt.
	prompt_renderer := NewPromptRenderer(cb.llmPromptTemplate, cb.llmBosToken, cb.llmAddGenerationPrompt)
	prompt, err := prompt_renderer.RenderPrompt(full_message_stack)

	if err != nil {
		log.Printf("<PROMPT RENDERER> Error while rendering prompt %s, use raw message instead.\n", err)
		return messages, err
	}

	return prompt, nil
}

// # Method: botReplyLoop
//
// Main bot reply loop.
func (cb *MainChatbot) botReplyLoop(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-cb.MessageReplyQueue:
			// Just print to stdout for now.
			// Sample a message.
			log.Printf("<REPLY> Message to reply: %s\n", msg)

			// Craft prompt.
			prompt, err := cb.replyPromptCrafter(msg.Message)
			if err != nil {
				// Ignore error, we have handled it in previous step.
				log.Println("<REPLY> Error while crafting prompt: ", err)
			}

			log.Println("<REPLY> Prompt crafted: ", prompt)

			bot_reply := BotMessage{
				Username: cb.chatbotName,
				Message:  msg.Message, // Dummy, connect to LLM model.
			}

			// Save history
			cb.appendChatHistory(PromptMessage{
				Role:    "user",
				Content: msg.Message,
			}, PromptMessage{
				Role:    "bot",
				Content: bot_reply.Message,
			})

			log.Println(irc.PRIVMSG(msg.Channel, msg.Message))
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

		llmPromptTemplate:      config.ChatbotSetting.LlmSetting.PromptSetting.PromptTemplate,
		llmBosToken:            config.ChatbotSetting.LlmSetting.PromptSetting.BosToken,
		llmAddGenerationPrompt: config.ChatbotSetting.LlmSetting.PromptSetting.AddGenerationPrompt,

		minReplyDelaySeconds:          config.ChatbotSetting.ReplySetting.ReplyMinDelaySeconds,
		maxReplyDelaySeconds:          config.ChatbotSetting.ReplySetting.ReplyMaxDelaySeconds,
		minReplyChatStallDelaySeconds: config.ChatbotSetting.ReplySetting.MinReplyChatStallDelaySeconds,
		maxReplyChatStallDelaySeconds: config.ChatbotSetting.ReplySetting.MaxReplyChatStallDelaySeconds,
	}

}
