package main

import (
	"github.com/kluctl/go-jinja2"
)

type PromptMessage struct {
	Role    string
	Content string
}

type PromptRenderer struct {
	PromptTemplate      string
	BosToken            string
	AddGenerationPrompt bool
}

func castMessageStructToMap(message PromptMessage) map[string]string {
	return map[string]string{
		"role":    message.Role,
		"content": message.Content,
	}
}

func castMessageListToMapList(messages []PromptMessage) []map[string]string {
	var result []map[string]string
	for _, message := range messages {
		result = append(result, castMessageStructToMap(message))
	}
	return result
}

func NewPromptRenderer(promptTemplate string, bosToken string, addGenerationPrompt bool) *PromptRenderer {
	return &PromptRenderer{
		PromptTemplate:      promptTemplate,
		BosToken:            bosToken,
		AddGenerationPrompt: addGenerationPrompt,
	}
}

func (pr *PromptRenderer) RenderPrompt(messages []PromptMessage) (string, error) {

	if len(messages) == 0 {
		return "", nil
	}

	j2, err := jinja2.NewJinja2("prompt", 1,
		jinja2.WithGlobal("bos_token", pr.BosToken),
		jinja2.WithGlobal("add_generation_prompt", pr.AddGenerationPrompt),
		jinja2.WithGlobal("messages", castMessageListToMapList(messages)),
	)

	if err != nil {
		return "", err
	}

	s, err := j2.RenderString(pr.PromptTemplate)

	if err != nil {
		return "", err
	}

	return s, nil
}
