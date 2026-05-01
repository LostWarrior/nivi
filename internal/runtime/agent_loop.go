package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LostWarrior/nivi/internal/config"
	nivierrors "github.com/LostWarrior/nivi/internal/errors"
	"github.com/LostWarrior/nivi/internal/provider"
)

const maxToolRounds = 8

func BuildMessages(systemPrompt string, history []provider.Message) []provider.Message {
	messages := make([]provider.Message, 0, len(history)+1)
	messages = append(messages, provider.Message{
		Role:    "system",
		Content: systemPrompt,
	})
	messages = append(messages, history...)
	return messages
}

func RunPlainChat(ctx context.Context, session Session, messages []provider.Message) (string, error) {
	request := provider.ChatRequest{
		Model:     session.Model,
		Messages:  messages,
		MaxTokens: session.Config.MaxTokens,
		Stream:    shouldStream(session.Config, session.IO),
	}
	if request.Stream {
		response, err := session.Client.Stream(ctx, request, func(delta string) error {
			_, writeErr := session.IO.Out.Write([]byte(delta))
			return writeErr
		})
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintln(session.IO.Out)
		return response, nil
	}
	return session.Client.Complete(ctx, request)
}

func ExecuteAgentTurn(
	ctx context.Context,
	session Session,
	history []provider.Message,
) (provider.Message, []provider.Message, error) {
	root, err := resolveSessionRoot(session)
	if err != nil {
		return provider.Message{}, nil, err
	}

	conversation := BuildMessages(session.Config.SystemPrompt, history)
	for round := 0; round < maxToolRounds; round++ {
		request := provider.ChatRequest{
			Model:      session.Model,
			Messages:   conversation,
			MaxTokens:  session.Config.MaxTokens,
			Tools:      agentTools(),
			ToolChoice: "auto",
		}

		turn, err := session.Client.CompleteTurn(ctx, request)
		if err != nil {
			return provider.Message{}, nil, err
		}

		assistant := provider.Message{
			Role:      "assistant",
			Content:   strings.TrimSpace(turn.Message.Content),
			ToolCalls: turn.Message.ToolCalls,
		}
		conversation = append(conversation, assistant)

		if len(assistant.ToolCalls) == 0 {
			if assistant.Content == "" {
				return provider.Message{}, nil, nivierrors.Protocol(
					"runtime.execute_agent_turn",
					"assistant returned empty output without tool calls",
					nil,
				)
			}
			return assistant, conversation[1:], nil
		}

		toolMessages := make([]provider.Message, 0, len(assistant.ToolCalls))
		for _, call := range assistant.ToolCalls {
			toolMessages = append(toolMessages, executeToolCall(session, root, call))
		}
		conversation = append(conversation, toolMessages...)
	}

	return provider.Message{}, nil, nivierrors.Protocol(
		"runtime.execute_agent_turn",
		fmt.Sprintf("tool loop exceeded %d rounds", maxToolRounds),
		nil,
	)
}

func shouldStream(state config.State, streams IO) bool {
	return state.StreamingEnabled && streams.StdoutTTY
}

func resolveSessionRoot(session Session) (string, error) {
	candidate := strings.TrimSpace(session.WorkspaceRoot)
	if candidate == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		candidate = cwd
	}
	return filepath.Abs(candidate)
}
