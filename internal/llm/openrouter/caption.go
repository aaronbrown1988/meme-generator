// Package openrouter is the production adapter family for the llm ports
// backed by the OpenRouter API (https://openrouter.ai).
package openrouter

import (
	"context"
	"errors"
	"fmt"
	"time"

	orsdk "github.com/OpenRouterTeam/go-sdk"
	"github.com/OpenRouterTeam/go-sdk/models/components"

	"meme-generator/internal/meme"
)

// captionTimeout bounds a single OpenRouter chat request. Without it, a
// stalled upstream leaves the generation stuck in "processing" forever
// with no feedback to the user.
const captionTimeout = 45 * time.Second

// CaptionAdapter satisfies meme.CaptionWriter by sending a chat
// completion request to OpenRouter and parsing the JSON caption out of
// the response text.
type CaptionAdapter struct {
	Client *orsdk.OpenRouter
	Model  string
}

func (a *CaptionAdapter) Write(userPrompt, systemPrompt string) (meme.Caption, error) {
	messages := []components.ChatMessages{}
	if systemPrompt != "" {
		messages = append(messages, components.CreateChatMessagesSystem(components.ChatSystemMessage{
			Content: components.CreateChatSystemMessageContentStr(systemPrompt),
			Role:    components.ChatSystemMessageRoleSystem,
		}))
	}
	messages = append(messages, components.CreateChatMessagesUser(components.ChatUserMessage{
		Content: components.CreateChatUserMessageContentStr(meme.BuildCaptionPrompt(userPrompt, "")),
		Role:    components.ChatUserMessageRoleUser,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), captionTimeout)
	defer cancel()

	model := a.Model
	res, err := a.Client.Chat.Send(ctx, components.ChatRequest{
		Model:    &model,
		Messages: messages,
	}, nil)
	if err != nil {
		return meme.Caption{}, wrapRequestError("caption", a.Model, captionTimeout, err)
	}
	if res.ChatResult == nil || len(res.ChatResult.Choices) == 0 {
		return meme.Caption{}, fmt.Errorf("openrouter returned no choices (model=%s)", a.Model)
	}

	content, ok := res.ChatResult.Choices[0].Message.Content.Get()
	if !ok || content == nil || content.Str == nil {
		return meme.Caption{}, fmt.Errorf("openrouter returned no text content (model=%s)", a.Model)
	}

	c, err := meme.ParseCaptionJSON(*content.Str)
	if err != nil {
		return meme.Caption{}, fmt.Errorf("parse caption JSON (model=%s): %w (output: %s)", a.Model, err, *content.Str)
	}
	return c, nil
}

// wrapRequestError produces a descriptive error for an OpenRouter
// request failure, calling out a timeout explicitly since
// context.DeadlineExceeded on its own gives no indication of what
// actually happened or which model was involved.
func wrapRequestError(step, model string, timeout time.Duration, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("openrouter %s request timed out after %s (model=%s): %w", step, timeout, model, err)
	}
	return fmt.Errorf("openrouter %s request failed (model=%s): %w", step, model, err)
}
