package main

type Message struct {
	Role    string
	Content string
}

type PromptRenderer struct {
	PromptTemplate      string
	BosToken            string
	AddGenerationPrompt bool
}

func castMessageStructToMap(message Message) map[string]string {
	return map[string]string{
		"role":    message.Role,
		"content": message.Content,
	}
}

func castMessageListToMapList(messages []Message) []map[string]string {
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
