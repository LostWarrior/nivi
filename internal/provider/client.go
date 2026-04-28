package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/LostWarrior/nivi/internal/config"
	nivierrors "github.com/LostWarrior/nivi/internal/errors"
)

const (
	userAgent          = "nivi/v1"
	listModelsPath     = "/models"
	listModelsTimeout  = 15 * time.Second
	listModelsAttempts = 2
	maxErrorBodyBytes  = 4 << 10
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type modelListResponse struct {
	Data []Model `json:"data"`
}

type apiErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func New(state config.State) (*Client, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}

	return &Client{
		baseURL: strings.TrimRight(state.BaseURL, "/"),
		apiKey:  state.APIKey,
		httpClient: &http.Client{
			Transport: transport,
		},
	}, nil
}

func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	var lastErr error

	for attempt := 1; attempt <= listModelsAttempts; attempt++ {
		models, err := c.listModelsOnce(ctx)
		if err == nil {
			return models, nil
		}

		lastErr = err
		if attempt == listModelsAttempts || !retryableListModelsError(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *Client) listModelsOnce(ctx context.Context) ([]Model, error) {
	requestCtx, cancel := context.WithTimeout(ctx, listModelsTimeout)
	defer cancel()

	request, err := c.newRequest(requestCtx, http.MethodGet, listModelsPath, nil, "application/json")
	if err != nil {
		return nil, err
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, classifyTransportError("provider.list_models", err)
	}
	defer response.Body.Close()

	if err := decodeAPIError(response, "provider.list_models", ""); err != nil {
		return nil, err
	}

	var payload modelListResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, nivierrors.Protocol(
			"provider.list_models",
			"NVIDIA API returned an unreadable model list.",
			err,
		)
	}

	models := normalizeModels(payload.Data)
	if len(models) == 0 {
		return nil, nivierrors.Unavailable(
			"provider.list_models",
			"no models are available to this API key.",
			nil,
		)
	}

	return models, nil
}

func (c *Client) newRequest(
	ctx context.Context,
	method string,
	path string,
	body io.Reader,
	accept string,
) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, nivierrors.Validation("provider.new_request", "failed to create NVIDIA API request")
	}

	request.Header.Set("Accept", accept)
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("User-Agent", userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	return request, nil
}

func decodeAPIError(response *http.Response, op string, requestedModel string) error {
	if response.StatusCode < http.StatusBadRequest {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	detail := parseAPIError(body)

	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nivierrors.Auth(op)
	case http.StatusBadRequest:
		if requestedModel != "" && looksLikeModelError(detail) {
			return nivierrors.InvalidModel(op, requestedModel)
		}
		return nivierrors.Validation(op, safeHTTPMessage(response.StatusCode, detail))
	case http.StatusNotFound:
		if requestedModel != "" && looksLikeModelError(detail) {
			return nivierrors.InvalidModel(op, requestedModel)
		}
		return nivierrors.Unavailable(
			op,
			"NVIDIA API endpoint was not found. Check NIVI_BASE_URL.",
			nil,
		)
	default:
		return nivierrors.Unavailable(op, safeHTTPMessage(response.StatusCode, detail), nil)
	}
}

func parseAPIError(body []byte) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return ""
	}

	var payload apiErrorEnvelope
	if err := json.Unmarshal(body, &payload); err == nil {
		if message := strings.TrimSpace(payload.Error.Message); message != "" {
			return nivierrors.RedactSecrets(message)
		}
	}

	return nivierrors.RedactSecrets(string(body))
}

func looksLikeModelError(detail string) bool {
	detail = strings.ToLower(detail)
	return strings.Contains(detail, "model") && (strings.Contains(detail, "invalid") ||
		strings.Contains(detail, "not found") ||
		strings.Contains(detail, "unknown") ||
		strings.Contains(detail, "available"))
}

func safeHTTPMessage(statusCode int, detail string) string {
	switch {
	case statusCode >= 500:
		return fmt.Sprintf("NVIDIA API returned HTTP %d. Try again shortly.", statusCode)
	case detail == "":
		return fmt.Sprintf("NVIDIA API returned HTTP %d.", statusCode)
	default:
		return fmt.Sprintf("NVIDIA API returned HTTP %d: %s", statusCode, detail)
	}
}

func classifyTransportError(op string, err error) error {
	if err == nil {
		return nil
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return nivierrors.Timeout(op, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		return nivierrors.Timeout(op, err)
	}
	return nivierrors.Network(op, err)
}

func normalizeModels(models []Model) []Model {
	unique := make(map[string]Model, len(models))
	for _, model := range models {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}
		model.ID = modelID
		unique[modelID] = model
	}

	normalized := make([]Model, 0, len(unique))
	for _, model := range unique {
		normalized = append(normalized, model)
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].ID < normalized[j].ID
	})

	return normalized
}

func retryableListModelsError(err error) bool {
	return nivierrors.IsKind(err, nivierrors.KindNetwork) ||
		nivierrors.IsKind(err, nivierrors.KindTimeout) ||
		nivierrors.IsKind(err, nivierrors.KindUnavailable)
}
