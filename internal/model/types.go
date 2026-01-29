// Package model provides the core data types and state management for the TUI.
package model

import (
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/turboline-ai/turbostream-tui/pkg/api"
)

// Screen represents the current screen in the TUI
type Screen int

const (
	ScreenLogin Screen = iota
	ScreenDashboard
	ScreenMarketplace
	ScreenFeedDetail
	ScreenRegisterFeed
	ScreenEditFeed
	ScreenFeeds
	ScreenAPI
	ScreenHelp
)

// Tab indices for main navigation
const (
	TabDashboard = iota
	TabRegisterFeed
	TabMyFeeds
	TabAPI
	TabHelp
	TabCount
)

// AIIntervalOptions for auto-query intervals in seconds
var AIIntervalOptions = []int{5, 10, 30, 60}

// FeedEntry is a simplified log line for feed updates
type FeedEntry struct {
	FeedID   string
	FeedName string
	Event    string
	Data     string
	Time     time.Time
}

// AIOutputEntry represents a single AI response in the output history
type AIOutputEntry struct {
	Response  string
	Timestamp time.Time
	Provider  string
	Duration  int64
}

// AuthState holds authentication-related state
type AuthState struct {
	Mode     string // "login" or "register"
	Email    textinput.Model
	Password textinput.Model
	Name     textinput.Model
	TOTP     textinput.Model
	Token    string
	User     *api.User
}

// NewAuthState creates a new AuthState with initialized inputs
func NewAuthState(presetEmail string) AuthState {
	email := textinput.New()
	email.Placeholder = ""
	email.SetValue(presetEmail)
	email.Focus()

	password := textinput.New()
	password.Placeholder = ""
	password.EchoMode = textinput.EchoPassword
	password.CharLimit = 64

	name := textinput.New()
	name.Placeholder = ""

	totp := textinput.New()
	totp.Placeholder = ""
	totp.CharLimit = 10

	return AuthState{
		Mode:     "login",
		Email:    email,
		Password: password,
		Name:     name,
		TOTP:     totp,
	}
}

// FeedFormState holds feed registration/edit form state
type FeedFormState struct {
	Name         textinput.Model
	Description  textinput.Model
	URL          textinput.Model
	Category     textinput.Model
	EventName    textinput.Model
	SubMsg       textinput.Model
	SystemPrompt textinput.Model
	FocusIndex   int
}

// NewFeedFormState creates a new FeedFormState with initialized inputs
func NewFeedFormState() FeedFormState {
	name := textinput.New()
	name.Placeholder = ""
	name.CharLimit = 100

	description := textinput.New()
	description.Placeholder = ""
	description.CharLimit = 500

	url := textinput.New()
	url.Placeholder = ""
	url.CharLimit = 500

	category := textinput.New()
	category.Placeholder = ""
	category.CharLimit = 50

	eventName := textinput.New()
	eventName.Placeholder = ""
	eventName.CharLimit = 100

	subMsg := textinput.New()
	subMsg.Placeholder = ""
	subMsg.CharLimit = 1000

	systemPrompt := textinput.New()
	systemPrompt.Placeholder = ""
	systemPrompt.CharLimit = 2000

	return FeedFormState{
		Name:         name,
		Description:  description,
		URL:          url,
		Category:     category,
		EventName:    eventName,
		SubMsg:       subMsg,
		SystemPrompt: systemPrompt,
		FocusIndex:   0,
	}
}

// Inputs returns the form inputs in order
func (f *FeedFormState) Inputs() []*textinput.Model {
	return []*textinput.Model{
		&f.Name,
		&f.Description,
		&f.URL,
		&f.Category,
		&f.EventName,
		&f.SubMsg,
		&f.SystemPrompt,
	}
}

// Labels returns the form labels in order
func (f *FeedFormState) Labels() []string {
	return []string{
		"Feed Name *",
		"Description",
		"WebSocket URL *",
		"Category",
		"Event Name",
		"Subscription Message (JSON)",
		"AI System Prompt",
	}
}

// BlurAll blurs all form inputs
func (f *FeedFormState) BlurAll() {
	f.Name.Blur()
	f.Description.Blur()
	f.URL.Blur()
	f.Category.Blur()
	f.EventName.Blur()
	f.SubMsg.Blur()
	f.SystemPrompt.Blur()
}

// Clear resets all form values
func (f *FeedFormState) Clear() {
	f.Name.SetValue("")
	f.Description.SetValue("")
	f.URL.SetValue("")
	f.Category.SetValue("")
	f.EventName.SetValue("")
	f.SubMsg.SetValue("")
	f.SystemPrompt.SetValue("")
	f.FocusIndex = 0
}

// SetFromFeed populates form fields from a feed
func (f *FeedFormState) SetFromFeed(feed api.Feed) {
	f.Name.SetValue(feed.Name)
	f.Description.SetValue(feed.Description)
	f.URL.SetValue(feed.URL)
	f.Category.SetValue(feed.Category)
	f.EventName.SetValue(feed.EventName)
	f.SubMsg.SetValue("")
	f.SystemPrompt.SetValue(feed.SystemPrompt)
	f.FocusIndex = 0
}

// AIState holds per-feed AI analysis state
type AIState struct {
	Prompts         map[string]textarea.Model  // feedID -> prompt input
	AutoMode        bool                       // auto or manual query mode
	Interval        int                        // seconds between auto queries
	IntervalIdx     int                        // index into AIIntervalOptions
	Responses       map[string]string          // feedID -> current streaming response
	OutputHistories map[string][]AIOutputEntry // feedID -> history of outputs
	Loading         map[string]bool            // feedID -> whether query is in progress
	Paused          map[string]bool            // feedID -> whether AI is paused
	LastQuery       map[string]time.Time       // feedID -> last query time
	Focused         bool                       // whether AI panel is focused for editing
	RequestID       string                     // current request ID (for selected feed)
	RequestFeedID   string                     // which feed the current request is for
	ActiveRequests  map[string]string          // requestID -> feedID (concurrent tracking)
	StartTimes      map[string]time.Time       // feedID -> when request started
	FirstTokens     map[string]time.Time       // feedID -> when first token was received
}

// NewAIState creates a new AIState with initialized maps
func NewAIState() AIState {
	return AIState{
		Prompts:         make(map[string]textarea.Model),
		AutoMode:        false,
		Interval:        10,
		IntervalIdx:     1, // 10 seconds default
		Responses:       make(map[string]string),
		OutputHistories: make(map[string][]AIOutputEntry),
		Loading:         make(map[string]bool),
		Paused:          make(map[string]bool),
		LastQuery:       make(map[string]time.Time),
		ActiveRequests:  make(map[string]string),
		StartTimes:      make(map[string]time.Time),
		FirstTokens:     make(map[string]time.Time),
	}
}

// GetOrCreatePrompt gets or creates a prompt for a feed
func (a *AIState) GetOrCreatePrompt(feedID string) textarea.Model {
	if prompt, ok := a.Prompts[feedID]; ok {
		return prompt
	}
	// Create new prompt for this feed
	newPrompt := textarea.New()
	newPrompt.Placeholder = "Enter a prompt to start AI analysis..."
	newPrompt.SetWidth(50)
	newPrompt.SetHeight(3)
	newPrompt.ShowLineNumbers = false
	newPrompt.Prompt = ""
	a.Prompts[feedID] = newPrompt
	return newPrompt
}

// GetPrompt returns the prompt for a feed or a default view-only version
func (a AIState) GetPrompt(feedID string) textarea.Model {
	if prompt, ok := a.Prompts[feedID]; ok {
		return prompt
	}
	// Return a new prompt for display purposes only
	newPrompt := textarea.New()
	newPrompt.Placeholder = "Enter a prompt to start AI analysis..."
	newPrompt.SetWidth(50)
	newPrompt.SetHeight(3)
	newPrompt.ShowLineNumbers = false
	return newPrompt
}

// CycleInterval cycles to the next AI interval option
func (a *AIState) CycleInterval() {
	a.IntervalIdx = (a.IntervalIdx + 1) % len(AIIntervalOptions)
	a.Interval = AIIntervalOptions[a.IntervalIdx]
}

// AddToHistory adds an AI response to the output history for a feed
func (a *AIState) AddToHistory(feedID string, entry AIOutputEntry) {
	history := a.OutputHistories[feedID]
	history = append(history, entry)
	// Keep only last 10 outputs
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	a.OutputHistories[feedID] = history
}

// UIState holds UI-related state
type UIState struct {
	TermWidth  int
	TermHeight int
	Loading    bool
	HelpPage   int
	HelpScroll int
}

// NewUIState creates a new UIState with defaults
func NewUIState() UIState {
	return UIState{
		TermWidth:  120,
		TermHeight: 40,
	}
}

// FeedDataState holds feed-related data
type FeedDataState struct {
	Feeds             []api.Feed
	Subscriptions     []api.Subscription
	SelectedIdx       int
	SelectedFeed      *api.Feed
	ActiveFeedID      string
	Entries           map[string][]FeedEntry
	DashboardSelected int
}

// NewFeedDataState creates a new FeedDataState with initialized maps
func NewFeedDataState() FeedDataState {
	return FeedDataState{
		Entries: make(map[string][]FeedEntry),
	}
}

// IsSubscribed checks if a feed is subscribed
func (f *FeedDataState) IsSubscribed(feedID string) bool {
	for _, s := range f.Subscriptions {
		if s.FeedID == feedID {
			return true
		}
	}
	return false
}

// AddEntry adds a feed entry, maintaining a maximum of 50 entries per feed
func (f *FeedDataState) AddEntry(feedID string, entry FeedEntry) int {
	entries := f.Entries[feedID]
	entries = append([]FeedEntry{entry}, entries...)

	evicted := 0
	if len(entries) > 50 {
		evicted = len(entries) - 50
		entries = entries[:50]
	}
	f.Entries[feedID] = entries
	return evicted
}

// MessageState holds status and error messages
type MessageState struct {
	Status string
	Error  string
}
