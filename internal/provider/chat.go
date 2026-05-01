package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

const (
	chatCompletionsPath     = "/chat/completions"
	chatCompletionTimeout   = 2 * time.Minute
	streamCompletionTimeout = 5 * time.Minute
	maxPromptBytes          = 1 << 20
	maxStreamTokenBytes     = 1 << 20
)

func (c *Client) Complete(ctx context.Context, chatRequest ChatRequest) (string, error) {
	turn, err := c.CompleteTurn(ctx, chatRequest)
	if err != nil {
		return "", err
	}
	if len(turn.Message.ToolCalls) > 0 {
		return "", nivierrors.Protocol(
			"provider.complete",
			"assistant returned tool calls where plain text was expected",
			nil,
		)
	}
	return turn.Message.Content, nil
}

func (c *Client) CompleteTurn(ctx context.Context, chatRequest ChatRequest) (AssistantTurn, error) {
	if err := validateChatRequest(chatRequest); err != nil {
		return AssistantTurn{}, err
	}

	chatRequest.Stream = false

	requestBody, err := json.Marshal(chatRequest)
	if err != nil {
		return AssistantTurn{}, nivierrors.Validation("provider.complete_turn", "failed to encode chat request")
	}

	requestCtx, cancel := context.WithTimeout(ctx, chatCompletionTimeout)
	defer cancel()

	request, err := c.newRequest(
		requestCtx,
		http.MethodPost,
		chatCompletionsPath,
		bytes.NewReader(requestBody),
		"application/json",
	)
	if err != nil {
		return AssistantTurn{}, err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return AssistantTurn{}, classifyTransportError("provider.complete_turn", err)
	}
	defer response.Body.Close()

	if err := decodeAPIError(response, "provider.complete_turn", chatRequest.Model); err != nil {
		return AssistantTurn{}, err
	}

	var payload chatCompletionResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return AssistantTurn{}, nivierrors.Protocol(
			"provider.complete_turn",
			"NVIDIA API returned an unreadable response.",
			err,
		)
	}
	if len(payload.Choices) == 0 {
		return AssistantTurn{}, nivierrors.Protocol(
			"provider.complete_turn",
			"NVIDIA API response did not include any completion choices.",
			nil,
		)
	}

	return AssistantTurn{
		Message:      payload.Choices[0].Message,
		FinishReason: payload.Choices[0].FinishReason,
	}, nil
}

func (c *Client) Stream(
	ctx context.Context,
	chatRequest ChatRequest,
	onDelta func(string) error,
) (string, error) {
	if err := validateChatRequest(chatRequest); err != nil {
		return "", err
	}

	chatRequest.Stream = true

	requestBody, err := json.Marshal(chatRequest)
	if err != nil {
		return "", nivierrors.Validation("provider.stream", "failed to encode chat request")
	}

	requestCtx, cancel := context.WithTimeout(ctx, streamCompletionTimeout)
	defer cancel()

	request, err := c.newRequest(
		requestCtx,
		http.MethodPost,
		chatCompletionsPath,
		bytes.NewReader(requestBody),
		"text/event-stream",
	)
	if err != nil {
		return "", err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", classifyTransportError("provider.stream", err)
	}
	defer response.Body.Close()

	if err := decodeAPIError(response, "provider.stream", chatRequest.Model); err != nil {
		return "", err
	}
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		return "", nivierrors.Protocol(
			"provider.stream",
			"NVIDIA API returned a non-streaming response to a streaming request.",
			nil,
		)
	}

	return consumeSSE(response.Body, onDelta)
}

func validateChatRequest(chatRequest ChatRequest) error {
	if strings.TrimSpace(chatRequest.Model) == "" {
		return nivierrors.Validation("provider.validate_chat_request", "chat requests require a model id")
	}
	if len(chatRequest.Messages) == 0 {
		return nivierrors.Validation("provider.validate_chat_request", "chat requests require at least one message")
	}
	if chatRequest.MaxTokens < 0 {
		return nivierrors.Validation("provider.validate_chat_request", "max tokens cannot be negative")
	}

	totalBytes := 0
	for _, message := range chatRequest.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			return nivierrors.Validation("provider.validate_chat_request", "chat messages require a role")
		}

		switch role {
		case "system", "user", "assistant", "tool":
		default:
			return nivierrors.Validation(
				"provider.validate_chat_request",
				"chat message role must be system, user, assistant, or tool",
			)
		}

		if role == "tool" && strings.TrimSpace(message.ToolCallID) == "" {
			return nivierrors.Validation(
				"provider.validate_chat_request",
				"tool messages require tool_call_id",
			)
		}

		totalBytes += len(message.Content)
	}

	if totalBytes > maxPromptBytes {
		return nivierrors.Validation("provider.validate_chat_request", "prompt exceeds the 1 MiB size limit")
	}

	return nil
}

func consumeSSE(body io.Reader, onDelta func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64<<10), maxStreamTokenBytes)

	var builder strings.Builder
	dataLines := make([]string, 0, 4)
	receivedDone := false

	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}

		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]

		if payload == "[DONE]" {
			receivedDone = true
			return nil
		}

		var chunk chatCompletionResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nivierrors.Protocol(
				"provider.stream",
				"NVIDIA API returned malformed streaming data.",
				err,
			)
		}
		if len(chunk.Choices) == 0 {
			return nil
		}

		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			return nil
		}

		builder.WriteString(delta)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return err
			}
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flushEvent(); err != nil {
				return "", err
			}
			if receivedDone {
				return builder.String(), nil
			}
		case strings.HasPrefix(line, ":"):
			continue
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nivierrors.Protocol(
			"provider.stream",
			"NVIDIA API stream terminated unexpectedly.",
			err,
		)
	}
	if err := flushEvent(); err != nil {
		return "", err
	}
	if receivedDone {
		return builder.String(), nil
	}

	return "", nivierrors.Protocol(
		"provider.stream",
		"NVIDIA API stream ended before the completion finished.",
		nil,
	)
}
