package model

import (
	"time"

	"github.com/turboline-ai/turbostream-tui/pkg/api"
)

// Authentication messages
type (
	// AuthResultMsg is returned after login/register attempt
	AuthResultMsg struct {
		Token string
		User  *api.User
		Err   error
	}

	// MeResultMsg is returned when fetching current user
	MeResultMsg struct {
		User *api.User
		Err  error
	}
)

// Feed data messages
type (
	// FeedsMsg is returned when loading feeds
	FeedsMsg struct {
		Feeds []api.Feed
		Err   error
	}

	// SubsMsg is returned when loading subscriptions
	SubsMsg struct {
		Subs []api.Subscription
		Err  error
	}

	// FeedDetailMsg is returned when fetching a single feed
	FeedDetailMsg struct {
		Feed *api.Feed
		Err  error
	}

	// SubscribeResultMsg is returned after subscribe/unsubscribe
	SubscribeResultMsg struct {
		FeedID string
		Action string // "subscribe" or "unsubscribe"
		Err    error
	}

	// FeedDataMsg is received when live feed data arrives
	FeedDataMsg struct {
		FeedID    string
		FeedName  string
		EventName string
		Data      string
		Time      time.Time
	}

	// PacketDroppedMsg is received when a packet couldn't be parsed
	PacketDroppedMsg struct {
		FeedID string
		Reason string
	}

	// TokenUsageUpdateMsg is received when token usage is updated
	TokenUsageUpdateMsg struct {
		Usage *api.TokenUsage
	}
)

// Feed CRUD messages
type (
	// FeedCreateMsg is returned after creating a feed
	FeedCreateMsg struct {
		Feed *api.Feed
		Err  error
	}

	// FeedUpdateMsg is returned after updating a feed
	FeedUpdateMsg struct {
		Feed *api.Feed
		Err  error
	}

	// FeedDeleteMsg is returned after deleting a feed
	FeedDeleteMsg struct {
		FeedID string
		Err    error
	}
)

// WebSocket messages
type (
	// WSConnectedMsg is returned when WebSocket connection is established
	WSConnectedMsg struct {
		Err error
	}

	// WSStatusMsg is received for WebSocket status updates
	WSStatusMsg struct {
		Status string // "connected", "disconnected", "reconnecting"
		Err    error
	}
)

// AI messages
type (
	// AIResponseMsg is received when AI response is complete
	AIResponseMsg struct {
		RequestID string
		Answer    string
		Provider  string
		Duration  int64
		Err       error
	}

	// AITokenMsg is received for streaming AI tokens
	AITokenMsg struct {
		RequestID string
		Token     string
	}

	// AITickMsg triggers auto-query at interval
	AITickMsg struct{}
)

// Timer messages
type (
	// UserTickMsg triggers periodic user data refresh
	UserTickMsg struct{}

	// DashboardTickMsg triggers dashboard metrics refresh
	DashboardTickMsg struct{}
)
