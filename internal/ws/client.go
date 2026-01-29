// Package ws provides WebSocket client functionality for real-time data streaming.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/turboline-ai/turbostream-tui/internal/model"
	"github.com/turboline-ai/turbostream-tui/pkg/api"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// envelope is the message wrapper for WebSocket communication
type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Client wraps the websocket connection and streams messages into the Bubble Tea loop.
type Client struct {
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	incoming chan tea.Msg
	userID   string
}

// Dial establishes a WebSocket connection and registers the user.
func Dial(url, userID, userAgent string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		Subprotocols: []string{},
	})
	if err != nil {
		cancel()
		return nil, err
	}

	client := &Client{
		conn:     conn,
		ctx:      ctx,
		cancel:   cancel,
		incoming: make(chan tea.Msg, 32),
		userID:   userID,
	}

	// Register the user
	regPayload := map[string]interface{}{
		"userId":    userID,
		"userAgent": userAgent,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if err := wsjson.Write(ctx, conn, map[string]interface{}{
		"type":    "register-user",
		"payload": regPayload,
	}); err != nil {
		if closeErr := conn.Close(websocket.StatusInternalError, "register failed"); closeErr != nil {
			log.Printf("error closing connection after registration failure: %v", closeErr)
		}
		cancel()
		return nil, fmt.Errorf("register-user failed: %w", err)
	}

	go client.readLoop()
	return client, nil
}

// readLoop reads messages from the WebSocket and sends them to the incoming channel.
func (c *Client) readLoop() {
	defer close(c.incoming)

	for {
		var env envelope
		if err := wsjson.Read(c.ctx, c.conn, &env); err != nil {
			c.incoming <- model.WSStatusMsg{Status: "disconnected", Err: err}
			return
		}

		msg := c.parseMessage(env)
		if msg != nil {
			c.incoming <- msg
		}
	}
}

// parseMessage converts a WebSocket envelope to a Bubble Tea message.
func (c *Client) parseMessage(env envelope) tea.Msg {
	switch env.Type {
	case "registration-success":
		return model.WSStatusMsg{Status: "connected", Err: nil}

	case "feed-data":
		var payload struct {
			FeedID    string          `json:"feedId"`
			FeedName  string          `json:"feedName"`
			EventName string          `json:"eventName"`
			Data      json.RawMessage `json:"data"`
			Timestamp string          `json:"timestamp"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			ts, _ := time.Parse(time.RFC3339, payload.Timestamp)
			return model.FeedDataMsg{
				FeedID:    payload.FeedID,
				FeedName:  payload.FeedName,
				EventName: payload.EventName,
				Data:      string(payload.Data),
				Time:      ts,
			}
		}
		return model.PacketDroppedMsg{
			FeedID: "",
			Reason: "json_parse_error",
		}

	case "token-usage-update":
		var usage api.TokenUsage
		if err := json.Unmarshal(env.Payload, &usage); err == nil {
			return model.TokenUsageUpdateMsg{Usage: &usage}
		}

	case "subscription-success", "unsubscription-success":
		// No-op; REST already returns status
		return nil

	case "llm-response":
		var payload struct {
			RequestID  string `json:"requestId"`
			Answer     string `json:"answer"`
			Provider   string `json:"provider"`
			DurationMs int64  `json:"durationMs"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			return model.AIResponseMsg{
				RequestID: payload.RequestID,
				Answer:    payload.Answer,
				Provider:  payload.Provider,
				Duration:  payload.DurationMs,
			}
		}

	case "llm-token":
		var payload struct {
			RequestID string `json:"requestId"`
			Token     string `json:"token"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			return model.AITokenMsg{
				RequestID: payload.RequestID,
				Token:     payload.Token,
			}
		}

	case "llm-complete":
		var payload struct {
			RequestID  string `json:"requestId"`
			Answer     string `json:"answer"`
			Provider   string `json:"provider"`
			DurationMs int64  `json:"durationMs"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			return model.AIResponseMsg{
				RequestID: payload.RequestID,
				Answer:    payload.Answer,
				Provider:  payload.Provider,
				Duration:  payload.DurationMs,
			}
		}

	case "llm-error":
		var payload struct {
			RequestID string `json:"requestId"`
			Error     string `json:"error"`
		}
		if err := json.Unmarshal(env.Payload, &payload); err == nil {
			return model.AIResponseMsg{
				RequestID: payload.RequestID,
				Err:       errors.New(payload.Error),
			}
		}
	}

	// Unknown types are ignored
	return nil
}

// ListenCmd returns a command that waits for the next WebSocket message.
func (c *Client) ListenCmd() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-c.incoming
		if !ok {
			return model.WSStatusMsg{Status: "disconnected", Err: errors.New("ws closed")}
		}
		return msg
	}
}

// Subscribe sends a subscription request for a feed.
func (c *Client) Subscribe(feedID string) error {
	return c.send(map[string]interface{}{
		"type": "subscribe-feed",
		"payload": map[string]string{
			"feedId": feedID,
			"userId": c.userID,
		},
	})
}

// Unsubscribe sends an unsubscription request for a feed.
func (c *Client) Unsubscribe(feedID string) error {
	return c.send(map[string]interface{}{
		"type": "unsubscribe-feed",
		"payload": map[string]string{
			"feedId": feedID,
			"userId": c.userID,
		},
	})
}

// SendLLMQuery sends a query to the LLM service via WebSocket.
func (c *Client) SendLLMQuery(feedID, question, systemPrompt, requestID string) error {
	return c.send(map[string]interface{}{
		"type": "llm-query-stream",
		"payload": map[string]string{
			"feedId":       feedID,
			"question":     question,
			"systemPrompt": systemPrompt,
			"requestId":    requestID,
		},
	})
}

// SendLLMStreamQuery sends a streaming query to the LLM service.
func (c *Client) SendLLMStreamQuery(feedID, question, requestID string) error {
	return c.send(map[string]interface{}{
		"type": "llm-query-stream",
		"payload": map[string]string{
			"feedId":    feedID,
			"question":  question,
			"requestId": requestID,
		},
	})
}

// send writes a message to the WebSocket with a timeout.
func (c *Client) send(msg interface{}) error {
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	return wsjson.Write(ctx, c.conn, msg)
}

// Close cleanly closes the WebSocket connection.
func (c *Client) Close() {
	c.cancel()
	_ = c.conn.Close(websocket.StatusNormalClosure, "bye")
}
