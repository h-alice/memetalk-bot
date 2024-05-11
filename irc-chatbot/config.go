package main

import (
	"gopkg.in/yaml.v3"
)

// YAML comfiguration for Twitch IRC bot.
type TwitchIrcConfig struct {
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	ChannelList []string `yaml:"join-channels"`
}

type ChatbotSetting struct {
	ReplySetting ChatbotReplySetting `yaml:"reply-setting"`
	LlmSetting   ChatbotLlmSetting   `yaml:"llm-setting"`
}

type ChatbotReplySetting struct {
	ReplyMention                  bool `yaml:"reply-mention"`
	ReplyMinDelaySeconds          int  `yaml:"reply-min-delay-seconds"`
	ReplyMaxDelaySeconds          int  `yaml:"reply-max-delay-seconds"`
	MessageSampleQueueSize        int  `yaml:"message-sample-queue-size"`
	MinReplyChatStallDelaySeconds int  `yaml:"reply-min-chat-stall-delay-seconds"`
	MaxReplyChatStallDelaySeconds int  `yaml:"reply-max-chat-stall-delay-seconds"`
}

type ChatbotLlmModelApiSetting struct {
	ServerUrl string `yaml:"server-url"`
	Endpoint  string `yaml:"endpoint"`
}

type ChatbotLlmSetting struct {
	PromptSetting   ChatbotLlmPromptSetting   `yaml:"prompt-setting"`
	MaxContextSize  int                       `yaml:"max-context-size"`
	ModelApiSetting ChatbotLlmModelApiSetting `yaml:"model-api-setting"`
}

type ChatbotLlmPromptSetting struct {
	PromptTemplate      string `yaml:"prompt-template"`
	BosToken            string `yaml:"bos-token"`
	AddGenerationPrompt bool   `yaml:"add-generation-prompt"`
}

// Config root.
type Config struct {
	TwitchIrcConfig TwitchIrcConfig `yaml:"twitch-irc"`
	ChatbotSetting  ChatbotSetting  `yaml:"chatbot-setting"`
}

// Parse YAML configuration file.
func ParseConfig(data []byte) (Config, error) {
	config := Config{}
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}
