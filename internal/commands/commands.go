// Package commands provides Bubble Tea command factories for async operations.
package commands

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/turboline-ai/turbostream-tui/internal/model"
	"github.com/turboline-ai/turbostream-tui/internal/ws"
	"github.com/turboline-ai/turbostream-tui/pkg/api"
)

// Default timeouts for operations
const (
	AuthTimeout   = 10 * time.Second
	FetchTimeout  = 10 * time.Second
	ActionTimeout = 8 * time.Second
	CreateTimeout = 15 * time.Second
)

// Login authenticates a user
func Login(client *api.Client, email, password, totp string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), AuthTimeout)
		defer cancel()
		token, user, err := client.Login(ctx, email, password, totp)
		return model.AuthResultMsg{Token: token, User: user, Err: err}
	}
}

// Register creates a new user account
func Register(client *api.Client, email, password, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), AuthTimeout)
		defer cancel()
		token, user, err := client.Register(ctx, email, password, name)
		return model.AuthResultMsg{Token: token, User: user, Err: err}
	}
}

// FetchMe gets the current user
func FetchMe(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ActionTimeout)
		defer cancel()
		user, err := client.Me(ctx)
		return model.MeResultMsg{User: user, Err: err}
	}
}

// LoadInitialData loads feeds and subscriptions in parallel
func LoadInitialData(client *api.Client) tea.Cmd {
	return tea.Batch(LoadFeeds(client), LoadSubscriptions(client))
}

// LoadFeeds fetches the user's feeds
func LoadFeeds(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
		defer cancel()
		feeds, err := client.MyFeeds(ctx)
		return model.FeedsMsg{Feeds: feeds, Err: err}
	}
}

// LoadSubscriptions fetches the user's subscriptions
func LoadSubscriptions(client *api.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
		defer cancel()
		subs, err := client.Subscriptions(ctx)
		return model.SubsMsg{Subs: subs, Err: err}
	}
}

// FetchFeed gets details for a single feed
func FetchFeed(client *api.Client, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
		defer cancel()
		feed, err := client.Feed(ctx, id)
		return model.FeedDetailMsg{Feed: feed, Err: err}
	}
}

// Subscribe subscribes to a feed
func Subscribe(client *api.Client, feedID, userID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ActionTimeout)
		defer cancel()
		err := client.Subscribe(ctx, feedID)
		return model.SubscribeResultMsg{FeedID: feedID, Action: "subscribe", Err: err}
	}
}

// Unsubscribe unsubscribes from a feed
func Unsubscribe(client *api.Client, feedID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), ActionTimeout)
		defer cancel()
		err := client.Unsubscribe(ctx, feedID)
		return model.SubscribeResultMsg{FeedID: feedID, Action: "unsubscribe", Err: err}
	}
}

// CreateFeed creates a new feed
func CreateFeed(client *api.Client, name, description, url, category, eventName, subMsg, systemPrompt string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), CreateTimeout)
		defer cancel()
		feed, err := client.CreateFeed(ctx, name, description, url, category, eventName, subMsg, systemPrompt)
		return model.FeedCreateMsg{Feed: feed, Err: err}
	}
}

// UpdateFeed updates an existing feed
func UpdateFeed(client *api.Client, feedID string, updates map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), CreateTimeout)
		defer cancel()
		feed, err := client.UpdateFeed(ctx, feedID, updates)
		return model.FeedUpdateMsg{Feed: feed, Err: err}
	}
}

// DeleteFeed deletes a feed
func DeleteFeed(client *api.Client, feedID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
		defer cancel()
		err := client.DeleteFeed(ctx, feedID)
		return model.FeedDeleteMsg{FeedID: feedID, Err: err}
	}
}

// ConnectWS establishes a WebSocket connection
func ConnectWS(url, userID, userAgent string) tea.Cmd {
	return func() tea.Msg {
		client, err := ws.Dial(url, userID, userAgent)
		if err != nil {
			return model.WSConnectedMsg{Err: err}
		}
		return WSClientConnected{Client: client}
	}
}

// WSClientConnected is returned when WebSocket connection is established
type WSClientConnected struct {
	Client *ws.Client
}

// StartAIAutoQuery starts the AI auto-query ticker
func StartAIAutoQuery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return model.AITickMsg{} })
}

// UserRefreshTick schedules the next user data refresh
func UserRefreshTick() tea.Cmd {
	return tea.Tick(5*time.Minute, func(t time.Time) tea.Msg { return model.UserTickMsg{} })
}

// DashboardRefreshTick schedules the next dashboard refresh
func DashboardRefreshTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return model.DashboardTickMsg{} })
}
