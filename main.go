package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/turboline-ai/turbostream-tui/internal/commands"
	"github.com/turboline-ai/turbostream-tui/internal/metrics"
	"github.com/turboline-ai/turbostream-tui/internal/model"
	"github.com/turboline-ai/turbostream-tui/internal/ui"
	"github.com/turboline-ai/turbostream-tui/internal/ws"
	"github.com/turboline-ai/turbostream-tui/pkg/api"
)

// Model keeps the application state (Elm-style).
type Model struct {
	// Configuration
	backendURL string
	wsURL      string
	client     *api.Client

	// Navigation
	screen    model.Screen
	activeTab int

	// State components
	auth     model.AuthState
	feedForm model.FeedFormState
	ai       model.AIState
	ui       model.UIState
	feeds    model.FeedDataState
	messages model.MessageState

	// WebSocket
	wsClient *ws.Client
	wsStatus string

	// UI helpers
	spinner    spinner.Model
	aiViewport viewport.Model

	// Metrics
	metricsCollector *metrics.Collector
	dashboardMetrics metrics.DashboardMetrics
}

func main() {
	backendURL := getenvDefault("TURBOSTREAM_BACKEND_URL", "http://localhost:7210")
	wsURL := getenvDefault("TURBOSTREAM_WEBSOCKET_URL", "ws://localhost:7210/ws")
	token := os.Getenv("TURBOSTREAM_TOKEN")
	email := os.Getenv("TURBOSTREAM_EMAIL")

	client := api.NewClient(backendURL)
	if token != "" {
		client.SetToken(token)
	}

	m := newModel(client, backendURL, wsURL, token, email)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("failed to start TUI:", err)
		os.Exit(1)
	}
}

func newModel(client *api.Client, backendURL, wsURL, token, presetEmail string) Model {
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	authState := model.NewAuthState(presetEmail)
	authState.Token = token

	return Model{
		backendURL:       backendURL,
		wsURL:            wsURL,
		client:           client,
		screen:           model.ScreenLogin,
		activeTab:        model.TabDashboard,
		auth:             authState,
		feedForm:         model.NewFeedFormState(),
		ai:               model.NewAIState(),
		ui:               model.NewUIState(),
		feeds:            model.NewFeedDataState(),
		spinner:          sp,
		metricsCollector: metrics.NewCollector(),
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}
	if m.auth.Token != "" {
		m.ui.Loading = true
		cmds = append(cmds, commands.FetchMe(m.client))
	}
	cmds = append(cmds, commands.UserRefreshTick())
	cmds = append(cmds, commands.DashboardRefreshTick())
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.ui.TermWidth = msg.Width
		m.ui.TermHeight = msg.Height
		return m, nil

	case model.AuthResultMsg:
		return m.handleAuthResult(msg)

	case model.MeResultMsg:
		return m.handleMeResult(msg)

	case model.FeedsMsg:
		return m.handleFeedsResult(msg)

	case model.SubsMsg:
		return m.handleSubsResult(msg)

	case model.FeedDetailMsg:
		return m.handleFeedDetail(msg)

	case model.SubscribeResultMsg:
		return m.handleSubscribeResult(msg)

	case commands.WSClientConnected:
		return m.handleWSConnected(msg)

	case model.WSStatusMsg:
		return m.handleWSStatus(msg)

	case model.FeedDataMsg:
		return m.handleFeedData(msg)

	case model.PacketDroppedMsg:
		m.metricsCollector.RecordPacketLoss(msg.FeedID)
		return m, m.nextWSListen()

	case model.DashboardTickMsg:
		m.dashboardMetrics = m.metricsCollector.GetMetrics()
		m.dashboardMetrics.SelectedIdx = m.feeds.DashboardSelected
		return m, commands.DashboardRefreshTick()

	case model.FeedCreateMsg:
		return m.handleFeedCreate(msg)

	case model.FeedUpdateMsg:
		return m.handleFeedUpdate(msg)

	case model.FeedDeleteMsg:
		return m.handleFeedDelete(msg)

	case model.AIResponseMsg:
		return m.handleAIResponse(msg)

	case model.AITokenMsg:
		return m.handleAIToken(msg)

	case model.AITickMsg:
		return m.handleAITick()

	case model.UserTickMsg:
		if m.auth.Token != "" {
			return m, commands.FetchMe(m.client)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// Message handlers

func (m Model) handleAuthResult(msg model.AuthResultMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.auth.Token = msg.Token
	m.auth.User = msg.User
	m.client.SetToken(msg.Token)
	m.screen = model.ScreenDashboard
	m.messages.Status = "Logged in"
	return m, tea.Batch(
		commands.LoadInitialData(m.client),
		commands.ConnectWS(m.wsURL, m.auth.User.ID, m.userAgent()),
	)
}

func (m Model) handleMeResult(msg model.MeResultMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		m.screen = model.ScreenLogin
		return m, nil
	}
	m.auth.User = msg.User
	m.screen = model.ScreenDashboard
	m.messages.Status = "Session restored"
	return m, tea.Batch(
		commands.LoadInitialData(m.client),
		commands.ConnectWS(m.wsURL, m.auth.User.ID, m.userAgent()),
	)
}

func (m Model) handleFeedsResult(msg model.FeedsMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.feeds.Feeds = msg.Feeds
	m.messages.Error = ""
	for _, feed := range msg.Feeds {
		m.metricsCollector.InitFeed(feed.ID, feed.Name)
	}
	return m, nil
}

func (m Model) handleSubsResult(msg model.SubsMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.feeds.Subscriptions = msg.Subs
	if m.wsClient != nil {
		for _, sub := range m.feeds.Subscriptions {
			_ = m.wsClient.Subscribe(sub.FeedID)
		}
	}
	return m, nil
}

func (m Model) handleFeedDetail(msg model.FeedDetailMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.feeds.SelectedFeed = msg.Feed
	m.feeds.ActiveFeedID = msg.Feed.ID
	m.screen = model.ScreenFeedDetail
	m.messages.Error = ""
	return m, nil
}

func (m Model) handleSubscribeResult(msg model.SubscribeResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.messages.Error = ""
	action := msg.Action
	if len(action) > 0 {
		action = strings.ToUpper(action[:1]) + action[1:]
	}
	m.messages.Status = fmt.Sprintf("%s successful for feed %s", action, msg.FeedID)
	var cmds []tea.Cmd
	cmds = append(cmds, commands.LoadSubscriptions(m.client))
	if m.wsClient != nil {
		if msg.Action == "subscribe" {
			_ = m.wsClient.Subscribe(msg.FeedID)
		} else {
			_ = m.wsClient.Unsubscribe(msg.FeedID)
			delete(m.feeds.Entries, msg.FeedID)
		}
		cmds = append(cmds, m.wsClient.ListenCmd())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleWSConnected(msg commands.WSClientConnected) (tea.Model, tea.Cmd) {
	if msg.Client == nil {
		m.wsStatus = "disconnected"
		return m, nil
	}
	m.wsClient = msg.Client
	m.wsStatus = "connected"
	var cmds []tea.Cmd
	cmds = append(cmds, m.wsClient.ListenCmd())
	for _, sub := range m.feeds.Subscriptions {
		_ = m.wsClient.Subscribe(sub.FeedID)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleWSStatus(msg model.WSStatusMsg) (tea.Model, tea.Cmd) {
	m.wsStatus = msg.Status
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
	}
	if msg.Status == "disconnected" {
		m.wsClient = nil
		for _, feed := range m.feeds.Feeds {
			m.metricsCollector.RecordWSStatus(feed.ID, false)
		}
	} else if msg.Status == "connected" {
		for _, feed := range m.feeds.Feeds {
			m.metricsCollector.RecordWSStatus(feed.ID, true)
		}
	}
	return m, m.nextWSListen()
}

func (m Model) handleFeedData(msg model.FeedDataMsg) (tea.Model, tea.Cmd) {
	m.metricsCollector.InitFeed(msg.FeedID, msg.FeedName)
	m.metricsCollector.RecordMessage(msg.FeedID, len(msg.Data))
	m.metricsCollector.RecordWSStatus(msg.FeedID, true)

	entry := model.FeedEntry{
		FeedID:   msg.FeedID,
		FeedName: msg.FeedName,
		Event:    msg.EventName,
		Data:     msg.Data,
		Time:     msg.Time,
	}
	evicted := m.feeds.AddEntry(msg.FeedID, entry)
	if evicted > 0 {
		m.metricsCollector.RecordContextEviction(msg.FeedID, evicted)
	}

	// Update cache metrics
	entries := m.feeds.Entries[msg.FeedID]
	var cacheBytes uint64
	for _, e := range entries {
		cacheBytes += uint64(len(e.Data))
	}
	m.metricsCollector.RecordCacheStats(msg.FeedID, len(entries), cacheBytes, 0)

	return m, m.nextWSListen()
}

func (m Model) handleFeedCreate(msg model.FeedCreateMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.messages.Status = fmt.Sprintf("Feed '%s' created! Auto-subscribing...", msg.Feed.Name)
	m.messages.Error = ""
	m.feedForm.Clear()
	m.feeds.SelectedFeed = msg.Feed
	m.feeds.ActiveFeedID = msg.Feed.ID
	m.screen = model.ScreenDashboard
	m.activeTab = model.TabMyFeeds
	m.feeds.SelectedIdx = 0

	var cmds []tea.Cmd
	cmds = append(cmds, commands.LoadFeeds(m.client))
	if m.auth.User != nil {
		cmds = append(cmds, commands.Subscribe(m.client, msg.Feed.ID, m.auth.User.ID))
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleFeedUpdate(msg model.FeedUpdateMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.messages.Status = fmt.Sprintf("Feed '%s' updated successfully!", msg.Feed.Name)
	m.messages.Error = ""
	m.feedForm.Clear()
	m.screen = model.ScreenFeeds
	return m, commands.LoadFeeds(m.client)
}

func (m Model) handleFeedDelete(msg model.FeedDeleteMsg) (tea.Model, tea.Cmd) {
	m.ui.Loading = false
	if msg.Err != nil {
		m.messages.Error = msg.Err.Error()
		return m, nil
	}
	m.messages.Status = "Feed deleted successfully!"
	m.messages.Error = ""
	delete(m.feeds.Entries, msg.FeedID)
	if m.feeds.SelectedIdx >= len(m.feeds.Feeds)-1 && m.feeds.SelectedIdx > 0 {
		m.feeds.SelectedIdx--
	}
	return m, tea.Batch(commands.LoadFeeds(m.client), commands.LoadSubscriptions(m.client))
}

func (m Model) handleAIResponse(msg model.AIResponseMsg) (tea.Model, tea.Cmd) {
	feedID, exists := m.ai.ActiveRequests[msg.RequestID]
	if !exists {
		feedID = m.ai.RequestFeedID
		if feedID == "" && m.feeds.SelectedFeed != nil {
			feedID = m.feeds.SelectedFeed.ID
		}
	}
	delete(m.ai.ActiveRequests, msg.RequestID)

	m.ai.Loading[feedID] = false
	if msg.Err != nil {
		m.ai.Responses[feedID] = "Error: " + msg.Err.Error()
		m.ai.AddToHistory(feedID, model.AIOutputEntry{
			Response:  "Error: " + msg.Err.Error(),
			Timestamp: time.Now(),
			Provider:  "error",
		})
		if feedID != "" {
			m.metricsCollector.RecordLLMRequest(feedID, 0, 0, 0, 0, 0, true)
		}
		return m, m.nextWSListen()
	}

	m.ai.Responses[feedID] = msg.Answer
	m.messages.Status = fmt.Sprintf("AI response received (%s, %dms)", msg.Provider, msg.Duration)

	m.ai.AddToHistory(feedID, model.AIOutputEntry{
		Response:  msg.Answer,
		Timestamp: time.Now(),
		Provider:  msg.Provider,
		Duration:  msg.Duration,
	})

	// Record LLM metrics
	if feedID != "" {
		promptValue := ""
		if feedPrompt, ok := m.ai.Prompts[feedID]; ok {
			promptValue = feedPrompt.Value()
		}
		promptTokens := len(promptValue) / 4
		responseTokens := len(msg.Answer) / 4
		eventsInPrompt := len(m.feeds.Entries[feedID])

		var ttftMs, genTimeMs float64
		if firstToken, ok := m.ai.FirstTokens[feedID]; ok && !firstToken.IsZero() {
			if startTime, ok := m.ai.StartTimes[feedID]; ok && !startTime.IsZero() {
				ttftMs = float64(firstToken.Sub(startTime).Milliseconds())
			}
		}
		if startTime, ok := m.ai.StartTimes[feedID]; ok && !startTime.IsZero() {
			genTimeMs = float64(time.Since(startTime).Milliseconds())
		}

		m.metricsCollector.RecordLLMRequest(feedID, promptTokens, responseTokens, ttftMs, genTimeMs, eventsInPrompt, false)
		delete(m.ai.StartTimes, feedID)
		delete(m.ai.FirstTokens, feedID)
	}
	return m, m.nextWSListen()
}

func (m Model) handleAIToken(msg model.AITokenMsg) (tea.Model, tea.Cmd) {
	feedID, exists := m.ai.ActiveRequests[msg.RequestID]
	if !exists {
		if msg.RequestID == m.ai.RequestID {
			feedID = m.ai.RequestFeedID
		} else {
			return m, m.nextWSListen()
		}
	}

	if _, hasFirstToken := m.ai.FirstTokens[feedID]; !hasFirstToken && len(msg.Token) > 0 {
		m.ai.FirstTokens[feedID] = time.Now()
	}
	m.ai.Responses[feedID] += msg.Token
	m.ai.Loading[feedID] = true
	return m, m.nextWSListen()
}

func (m Model) handleAITick() (tea.Model, tea.Cmd) {
	if m.ai.AutoMode {
		var cmds []tea.Cmd
		for _, sub := range m.feeds.Subscriptions {
			feedID := sub.FeedID
			if m.ai.Paused[feedID] || m.ai.Loading[feedID] {
				continue
			}
			lastQuery, hasQuery := m.ai.LastQuery[feedID]
			if !hasQuery || time.Since(lastQuery) >= time.Duration(m.ai.Interval)*time.Second {
				m.ai.LastQuery[feedID] = time.Now()
				m.ai.Loading[feedID] = true
				requestID := fmt.Sprintf("req-%d-%s", time.Now().UnixNano(), feedID)

				if m.feeds.SelectedFeed != nil && m.feeds.SelectedFeed.ID == feedID {
					m.ai.RequestID = requestID
					m.ai.RequestFeedID = feedID
				}

				m.ai.ActiveRequests[requestID] = feedID
				m.ai.StartTimes[feedID] = time.Now()
				delete(m.ai.FirstTokens, feedID)
				m.ai.Responses[feedID] = ""

				cmds = append(cmds, m.sendAIQueryForFeed(feedID, requestID))
			}
		}
		cmds = append(cmds, m.nextWSListen(), commands.StartAIAutoQuery())
		return m, tea.Batch(cmds...)
	}
	return m, commands.StartAIAutoQuery()
}

// Key handling

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.String() == "ctrl+c" {
		if m.wsClient != nil {
			m.wsClient.Close()
		}
		return m, tea.Quit
	}

	// Quit on 'q' only if not in input mode
	if msg.String() == "q" {
		isInputMode := m.screen == model.ScreenLogin ||
			m.screen == model.ScreenRegisterFeed ||
			m.screen == model.ScreenEditFeed ||
			m.ai.Focused

		if !isInputMode {
			if m.wsClient != nil {
				m.wsClient.Close()
			}
			return m, tea.Quit
		}
	}

	if m.screen == model.ScreenLogin {
		return m.updateAuth(msg)
	}

	// Tab switching
	switch msg.String() {
	case "tab":
		m.activeTab = (m.activeTab + 1) % model.TabCount
		m.blurAllAIPrompts()
		m.ai.Focused = false
		return m.switchToTab(m.activeTab), nil
	case "shift+tab":
		m.activeTab--
		if m.activeTab < 0 {
			m.activeTab = model.TabCount - 1
		}
		m.blurAllAIPrompts()
		m.ai.Focused = false
		return m.switchToTab(m.activeTab), nil
	}

	// Screen-specific handling
	if m.screen == model.ScreenRegisterFeed {
		return m.updateRegisterFeed(msg)
	}
	if m.screen == model.ScreenEditFeed {
		return m.updateEditFeed(msg)
	}

	// AI prompt input
	if m.ai.Focused {
		return m.updateAIPrompt(msg)
	}

	// Dashboard navigation
	if m.screen == model.ScreenDashboard {
		switch msg.String() {
		case "up", "k":
			if len(m.dashboardMetrics.Feeds) > 0 {
				m.feeds.DashboardSelected--
				if m.feeds.DashboardSelected < 0 {
					m.feeds.DashboardSelected = len(m.dashboardMetrics.Feeds) - 1
				}
				m.dashboardMetrics.SelectedIdx = m.feeds.DashboardSelected
			}
			return m, nil
		case "down", "j":
			if len(m.dashboardMetrics.Feeds) > 0 {
				m.feeds.DashboardSelected++
				if m.feeds.DashboardSelected >= len(m.dashboardMetrics.Feeds) {
					m.feeds.DashboardSelected = 0
				}
				m.dashboardMetrics.SelectedIdx = m.feeds.DashboardSelected
			}
			return m, nil
		}
	}

	// Help navigation
	if m.screen == model.ScreenHelp {
		return m.updateHelp(msg)
	}

	// General key handling
	return m.handleGeneralKeys(msg)
}

func (m Model) handleGeneralKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.screen != model.ScreenDashboard && m.feeds.SelectedIdx > 0 {
			m.feeds.SelectedIdx--
		}
	case "down":
		if m.screen != model.ScreenDashboard && m.feeds.SelectedIdx < len(m.feeds.Feeds)-1 {
			m.feeds.SelectedIdx++
		}
	case "enter":
		if len(m.feeds.Feeds) > 0 {
			feed := m.feeds.Feeds[m.feeds.SelectedIdx]
			return m, commands.FetchFeed(m.client, feed.ID)
		}
	case "s":
		return m.handleSubscribeToggle()
	case "e":
		return m.handleEditFeed()
	case "D":
		return m.handleDeleteFeed()
	case "m":
		return m.handleToggleAIMode()
	case "i":
		return m.handleCycleInterval()
	case "P":
		return m.handleTogglePause()
	case "p":
		return m.handleFocusPrompt()
	case "esc":
		return m.handleEscape()
	case "r":
		return m.handleReconnect()
	case "l":
		return m.handleLogout()
	}
	return m, nil
}

func (m Model) handleSubscribeToggle() (tea.Model, tea.Cmd) {
	var feedID string
	var userID string
	if m.auth.User != nil {
		userID = m.auth.User.ID
	}
	if m.screen == model.ScreenFeeds && len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
		feedID = m.feeds.Feeds[m.feeds.SelectedIdx].ID
	} else if m.feeds.SelectedFeed != nil {
		feedID = m.feeds.SelectedFeed.ID
	}
	if feedID != "" && userID != "" {
		if m.feeds.IsSubscribed(feedID) {
			return m, commands.Unsubscribe(m.client, feedID)
		}
		return m, commands.Subscribe(m.client, feedID, userID)
	}
	return m, nil
}

func (m Model) handleEditFeed() (tea.Model, tea.Cmd) {
	if m.screen == model.ScreenFeeds && len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
		feed := m.feeds.Feeds[m.feeds.SelectedIdx]
		if m.auth.User != nil && feed.OwnerID == m.auth.User.ID {
			m.screen = model.ScreenEditFeed
			m.feedForm.SetFromFeed(feed)
			m.messages.Error = ""
			return m, m.feedForm.Name.Focus()
		}
		m.messages.Error = "You can only edit your own feeds"
	}
	return m, nil
}

func (m Model) handleDeleteFeed() (tea.Model, tea.Cmd) {
	if m.screen == model.ScreenFeeds && len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
		feed := m.feeds.Feeds[m.feeds.SelectedIdx]
		if m.auth.User != nil && feed.OwnerID == m.auth.User.ID {
			m.ui.Loading = true
			return m, commands.DeleteFeed(m.client, feed.ID)
		}
		m.messages.Error = "You can only delete your own feeds"
	}
	return m, nil
}

func (m Model) handleToggleAIMode() (tea.Model, tea.Cmd) {
	if (m.screen == model.ScreenFeeds || m.screen == model.ScreenDashboard) && !m.ai.Focused {
		m.ai.AutoMode = !m.ai.AutoMode
		if m.ai.AutoMode {
			m.messages.Status = fmt.Sprintf("AI Auto mode enabled (every %ds)", m.ai.Interval)
			for _, f := range m.feeds.Feeds {
				m.ai.LastQuery[f.ID] = time.Now().Add(-time.Duration(m.ai.Interval) * time.Second)
			}
			return m, commands.StartAIAutoQuery()
		}
		m.messages.Status = "AI Manual mode enabled"
	}
	return m, nil
}

func (m Model) handleCycleInterval() (tea.Model, tea.Cmd) {
	if (m.screen == model.ScreenFeeds || m.screen == model.ScreenDashboard) && !m.ai.Focused {
		m.ai.CycleInterval()
		m.messages.Status = fmt.Sprintf("AI query interval set to %ds", m.ai.Interval)
	}
	return m, nil
}

func (m Model) handleTogglePause() (tea.Model, tea.Cmd) {
	if (m.screen == model.ScreenFeeds || m.screen == model.ScreenDashboard) && !m.ai.Focused {
		if len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
			feedID := m.feeds.Feeds[m.feeds.SelectedIdx].ID
			m.ai.Paused[feedID] = !m.ai.Paused[feedID]
			if m.ai.Paused[feedID] {
				m.messages.Status = "AI Analysis PAUSED for this feed (Shift+P to resume)"
			} else {
				m.messages.Status = "AI Analysis RESUMED for this feed"
				if m.ai.AutoMode {
					m.ai.LastQuery[feedID] = time.Now().Add(-time.Duration(m.ai.Interval) * time.Second)
					return m, commands.StartAIAutoQuery()
				}
			}
		}
	}
	return m, nil
}

func (m Model) handleFocusPrompt() (tea.Model, tea.Cmd) {
	if (m.screen == model.ScreenFeeds || m.screen == model.ScreenDashboard) && !m.ai.Focused {
		m.ai.Focused = true
		if len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
			feedID := m.feeds.Feeds[m.feeds.SelectedIdx].ID
			prompt := m.ai.GetOrCreatePrompt(feedID)
			prompt.Focus()
			m.ai.Prompts[feedID] = prompt
		}
	}
	return m, nil
}

func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	if m.ai.Focused {
		m.ai.Focused = false
		if len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
			feedID := m.feeds.Feeds[m.feeds.SelectedIdx].ID
			if prompt, ok := m.ai.Prompts[feedID]; ok {
				prompt.Blur()
				m.ai.Prompts[feedID] = prompt
			}
		}
		return m, nil
	}
	if m.screen == model.ScreenFeedDetail {
		m.screen = model.ScreenFeeds
		m.feeds.SelectedFeed = nil
		return m, nil
	}
	return m, nil
}

func (m Model) handleReconnect() (tea.Model, tea.Cmd) {
	if m.auth.User != nil {
		if m.wsClient != nil {
			m.wsClient.Close()
			m.wsClient = nil
		}
		m.wsStatus = "reconnecting"
		return m, commands.ConnectWS(m.wsURL, m.auth.User.ID, m.userAgent())
	}
	return m, nil
}

func (m Model) handleLogout() (tea.Model, tea.Cmd) {
	if m.wsClient != nil {
		m.wsClient.Close()
	}
	m.auth.Token = ""
	m.auth.User = nil
	m.client.SetToken("")
	m.feeds.Feeds = nil
	m.feeds.Subscriptions = nil
	m.feeds.SelectedFeed = nil
	m.feeds.Entries = make(map[string]model.FeedEntry)
	m.wsClient = nil
	m.wsStatus = ""
	m.screen = model.ScreenLogin
	m.messages.Status = "Logged out"
	m.messages.Error = ""
	m.auth.Email.SetValue("")
	m.auth.Password.SetValue("")
	m.auth.Name.SetValue("")
	m.auth.TOTP.SetValue("")
	m.auth.Email.Focus()
	return m, nil
}

func (m Model) updateAuth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg.Type {
	case tea.KeyEnter:
		m.ui.Loading = true
		m.messages.Error = ""
		if m.auth.Mode == "login" {
			return m, commands.Login(m.client, m.auth.Email.Value(), m.auth.Password.Value(), m.auth.TOTP.Value())
		}
		return m, commands.Register(m.client, m.auth.Email.Value(), m.auth.Password.Value(), m.auth.Name.Value())
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyDown:
		cmds = append(cmds, m.switchAuthFocusNext())
		return m, tea.Batch(cmds...)
	case tea.KeyUp:
		cmds = append(cmds, m.switchAuthFocusPrev())
		return m, tea.Batch(cmds...)
	case tea.KeyCtrlS:
		if m.auth.Mode == "login" {
			m.auth.Mode = "register"
		} else {
			m.auth.Mode = "login"
		}
		return m, nil
	}

	var cmd tea.Cmd
	if m.auth.Email.Focused() {
		m.auth.Email, cmd = m.auth.Email.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.auth.Password.Focused() {
		m.auth.Password, cmd = m.auth.Password.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.auth.TOTP.Focused() {
		m.auth.TOTP, cmd = m.auth.TOTP.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.auth.Name.Focused() {
		m.auth.Name, cmd = m.auth.Name.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) switchAuthFocusNext() tea.Cmd {
	if m.auth.Email.Focused() {
		m.auth.Email.Blur()
		return m.auth.Password.Focus()
	}
	if m.auth.Password.Focused() {
		m.auth.Password.Blur()
		return m.auth.TOTP.Focus()
	}
	if m.auth.TOTP.Focused() {
		m.auth.TOTP.Blur()
		if m.auth.Mode == "register" {
			return m.auth.Name.Focus()
		}
		return m.auth.Email.Focus()
	}
	if m.auth.Name.Focused() {
		m.auth.Name.Blur()
		return m.auth.Email.Focus()
	}
	return m.auth.Email.Focus()
}

func (m *Model) switchAuthFocusPrev() tea.Cmd {
	if m.auth.Email.Focused() {
		m.auth.Email.Blur()
		if m.auth.Mode == "register" {
			return m.auth.Name.Focus()
		}
		return m.auth.TOTP.Focus()
	}
	if m.auth.Password.Focused() {
		m.auth.Password.Blur()
		return m.auth.Email.Focus()
	}
	if m.auth.TOTP.Focused() {
		m.auth.TOTP.Blur()
		return m.auth.Password.Focus()
	}
	if m.auth.Name.Focused() {
		m.auth.Name.Blur()
		return m.auth.TOTP.Focus()
	}
	return m.auth.Email.Focus()
}

func (m Model) updateRegisterFeed(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = model.ScreenDashboard
		m.feedForm.BlurAll()
		return m, nil
	case tea.KeyEnter:
		m.ui.Loading = true
		m.messages.Error = ""
		return m, commands.CreateFeed(m.client,
			m.feedForm.Name.Value(),
			m.feedForm.Description.Value(),
			m.feedForm.URL.Value(),
			m.feedForm.Category.Value(),
			m.feedForm.EventName.Value(),
			m.feedForm.SubMsg.Value(),
			m.feedForm.SystemPrompt.Value())
	case tea.KeyDown:
		return m, m.nextFeedFormFocus()
	case tea.KeyUp:
		return m, m.prevFeedFormFocus()
	}

	return m.updateFeedFormInput(msg)
}

func (m Model) updateEditFeed(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = model.ScreenFeeds
		m.messages.Error = ""
		return m, nil
	case tea.KeyEnter:
		if m.feedForm.Name.Value() == "" || m.feedForm.URL.Value() == "" {
			m.messages.Error = "Name and URL are required"
			return m, nil
		}
		m.ui.Loading = true
		m.messages.Error = ""
		updates := map[string]interface{}{
			"name":         m.feedForm.Name.Value(),
			"description":  m.feedForm.Description.Value(),
			"url":          m.feedForm.URL.Value(),
			"category":     m.feedForm.Category.Value(),
			"eventName":    m.feedForm.EventName.Value(),
			"systemPrompt": m.feedForm.SystemPrompt.Value(),
		}
		return m, commands.UpdateFeed(m.client, m.feeds.Feeds[m.feeds.SelectedIdx].ID, updates)
	case tea.KeyUp, tea.KeyShiftTab:
		return m, m.prevFeedFormFocus()
	case tea.KeyDown, tea.KeyTab:
		return m, m.nextFeedFormFocus()
	}

	return m.updateFeedFormInput(msg)
}

func (m Model) updateFeedFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	inputs := m.feedForm.Inputs()
	if m.feedForm.FocusIndex < len(inputs) {
		*inputs[m.feedForm.FocusIndex], cmd = inputs[m.feedForm.FocusIndex].Update(msg)
	}
	return m, cmd
}

func (m *Model) nextFeedFormFocus() tea.Cmd {
	inputs := m.feedForm.Inputs()
	inputs[m.feedForm.FocusIndex].Blur()
	m.feedForm.FocusIndex = (m.feedForm.FocusIndex + 1) % len(inputs)
	return inputs[m.feedForm.FocusIndex].Focus()
}

func (m *Model) prevFeedFormFocus() tea.Cmd {
	inputs := m.feedForm.Inputs()
	inputs[m.feedForm.FocusIndex].Blur()
	m.feedForm.FocusIndex--
	if m.feedForm.FocusIndex < 0 {
		m.feedForm.FocusIndex = len(inputs) - 1
	}
	return inputs[m.feedForm.FocusIndex].Focus()
}

func (m Model) updateAIPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var currentFeedID string
	if len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
		currentFeedID = m.feeds.Feeds[m.feeds.SelectedIdx].ID
	}

	switch msg.String() {
	case "esc":
		m.ai.Focused = false
		if currentFeedID != "" {
			if prompt, ok := m.ai.Prompts[currentFeedID]; ok {
				prompt.Blur()
				m.ai.Prompts[currentFeedID] = prompt
			}
		}
		return m, nil
	case "enter":
		m.ai.Focused = false
		if currentFeedID != "" {
			if prompt, ok := m.ai.Prompts[currentFeedID]; ok {
				prompt.Blur()
				m.ai.Prompts[currentFeedID] = prompt
			}
		}
		if len(m.feeds.Feeds) > 0 && m.feeds.SelectedIdx < len(m.feeds.Feeds) {
			feed := m.feeds.Feeds[m.feeds.SelectedIdx]
			if m.feeds.IsSubscribed(feed.ID) {
				if m.ai.Paused[feed.ID] {
					m.messages.Status = "AI is paused for this feed. Press 'P' to resume."
					return m, nil
				}
				m.feeds.SelectedFeed = &feed
				feedID := feed.ID
				m.ai.Loading[feedID] = true
				requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
				m.ai.RequestID = requestID
				m.ai.RequestFeedID = feedID
				m.ai.ActiveRequests[requestID] = feedID
				m.ai.StartTimes[feedID] = time.Now()
				delete(m.ai.FirstTokens, feedID)
				m.ai.Responses[feedID] = ""
				return m, tea.Batch(m.sendAIQuery(), m.nextWSListen())
			}
		}
		return m, nil
	default:
		if currentFeedID != "" {
			prompt := m.ai.GetOrCreatePrompt(currentFeedID)
			var cmd tea.Cmd
			prompt, cmd = prompt.Update(msg)
			m.ai.Prompts[currentFeedID] = prompt
			return m, cmd
		}
		return m, nil
	}
}

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		if m.ui.HelpPage > 0 {
			m.ui.HelpPage--
			m.ui.HelpScroll = 0
		}
	case "right", "l":
		m.ui.HelpPage++
		m.ui.HelpScroll = 0
	case "up", "k":
		if m.ui.HelpScroll > 0 {
			m.ui.HelpScroll--
		}
	case "down", "j":
		m.ui.HelpScroll++
	case "1", "2", "3", "4", "5":
		pageNum := int(msg.String()[0] - '1')
		m.ui.HelpPage = pageNum
		m.ui.HelpScroll = 0
	}
	return m, nil
}

func (m Model) switchToTab(tab int) Model {
	switch tab {
	case model.TabDashboard:
		m.screen = model.ScreenDashboard
	case model.TabRegisterFeed:
		m.screen = model.ScreenRegisterFeed
		m.feedForm.Name.Focus()
		m.feedForm.FocusIndex = 0
	case model.TabMyFeeds:
		m.screen = model.ScreenFeeds
	case model.TabAPI:
		m.screen = model.ScreenAPI
	case model.TabHelp:
		m.screen = model.ScreenHelp
	}
	return m
}

func (m *Model) blurAllAIPrompts() {
	for feedID, prompt := range m.ai.Prompts {
		prompt.Blur()
		m.ai.Prompts[feedID] = prompt
	}
}

// View renders the UI

func (m Model) View() string {
	if m.screen == model.ScreenLogin {
		return m.viewAuth()
	}
	return m.viewApp()
}

func (m Model) viewAuth() string {
	var builder strings.Builder

	builder.WriteString(ui.RenderGradientLogo())
	builder.WriteString("\n")

	if m.auth.Mode == "login" {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render("Login"))
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(" (Ctrl+S for register)"))
	} else {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render("Register"))
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(" (Ctrl+S for login)"))
	}
	builder.WriteString("\n\n")

	if m.auth.Mode == "register" {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Name: "))
		builder.WriteString(m.auth.Name.View())
		builder.WriteString("\n")
	}
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Email: "))
	builder.WriteString(m.auth.Email.View())
	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Password: "))
	builder.WriteString(m.auth.Password.View())
	builder.WriteString("\n")
	if m.auth.Mode == "login" {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("TOTP (optional): "))
		builder.WriteString(m.auth.TOTP.View())
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Enter to submit | Up/Down navigate | q to quit"))

	if m.ui.Loading {
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("%s Authenticating...", m.spinner.View()))
	}
	if m.messages.Error != "" {
		builder.WriteString("\n")
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.RedColor).Render(m.messages.Error))
	}

	return ui.BoxStyle.Render(builder.String())
}

func (m Model) viewApp() string {
	top := m.viewTopBar()
	tabBar := m.viewTabBar()
	content := m.viewContent()
	footer := m.viewFooter()
	return lipgloss.JoinVertical(lipgloss.Left, top, tabBar, content, footer)
}

func (m Model) viewTopBar() string {
	left := lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("TurboStream")
	status := fmt.Sprintf("Backend: %s | WS: %s", m.backendURL, m.wsStatus)
	if m.auth.User != nil && m.auth.User.TokenUsage != nil {
		status += fmt.Sprintf(" | Tokens %d/%d", m.auth.User.TokenUsage.TokensUsed, m.auth.User.TokenUsage.Limit)
	}
	userInfo := ""
	if m.auth.User != nil {
		userInfo = lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(fmt.Sprintf(" | %s [l to logout]", m.auth.User.Email))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", status, userInfo)
}

func (m Model) viewTabBar() string {
	tabs := []string{"Dashboard", "Register Feed", "My Feeds", "API", "Help"}
	var renderedTabs []string

	for i, tab := range tabs {
		if i == m.activeTab {
			renderedTabs = append(renderedTabs, ui.ActiveTabStyle.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, ui.InactiveTabStyle.Render(tab))
		}
	}

	tabRow := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	return ui.TabBarStyle.Render(tabRow)
}

func (m Model) viewContent() string {
	switch m.screen {
	case model.ScreenDashboard:
		return m.viewDashboard()
	case model.ScreenFeedDetail:
		return m.viewFeedDetail()
	case model.ScreenRegisterFeed:
		return m.viewRegisterFeed()
	case model.ScreenEditFeed:
		return m.viewEditFeed()
	case model.ScreenFeeds:
		return m.viewMyFeeds()
	case model.ScreenAPI:
		return m.viewAPI()
	case model.ScreenHelp:
		return m.viewHelp()
	default:
		return ""
	}
}

func (m Model) viewDashboard() string {
	if len(m.dashboardMetrics.Feeds) > 0 {
		return ui.RenderDashboardView(m.dashboardMetrics, m.ui.TermWidth, m.ui.TermHeight)
	}

	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("Observability Dashboard"))
	builder.WriteString("\n\n")

	stats := []string{
		fmt.Sprintf("Total Feeds: %d", len(m.feeds.Feeds)),
		fmt.Sprintf("Active Subscriptions: %d", len(m.feeds.Subscriptions)),
	}
	if m.auth.User != nil && m.auth.User.TokenUsage != nil {
		stats = append(stats, fmt.Sprintf("Token Usage: %d/%d", m.auth.User.TokenUsage.TokensUsed, m.auth.User.TokenUsage.Limit))
	}

	for _, stat := range stats {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("* "))
		builder.WriteString(stat)
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Subscribe to a feed to see streaming metrics."))
	builder.WriteString("\n\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Tab/Shift+Tab: switch tabs | h/l: prev/next feed | q: quit"))

	return ui.ContentStyle.Render(builder.String())
}

func (m Model) viewMyFeeds() string {
	if len(m.feeds.Feeds) == 0 {
		var builder strings.Builder
		builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("My Feeds"))
		builder.WriteString("\n\n")
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("No feeds registered yet. Use 'Register Feed' tab to add a WebSocket feed!"))
		return ui.ContentStyle.Render(builder.String())
	}

	// Layout dimensions
	leftColWidth := 35
	middleColWidth := 60
	margin := 2
	rightMargin := 6
	usedWidth := leftColWidth + margin + middleColWidth + margin + rightMargin
	aiColWidth := m.ui.TermWidth - usedWidth
	if aiColWidth < 40 {
		aiColWidth = 40
	}

	feedListHeight := 12
	streamHeight := 25
	infoBoxHeight := 10
	instructHeight := infoBoxHeight + streamHeight - feedListHeight
	if instructHeight < 8 {
		instructHeight = 8
	}
	aiHeight := infoBoxHeight + streamHeight + 2

	// Feed list
	feedListBox := m.renderFeedList(leftColWidth, feedListHeight)
	instructBox := m.renderInstructions(leftColWidth, instructHeight)
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, feedListBox, instructBox)

	// Middle and right content
	var rightBuilder strings.Builder
	if m.feeds.SelectedIdx < len(m.feeds.Feeds) {
		feed := m.feeds.Feeds[m.feeds.SelectedIdx]

		infoBox := m.renderFeedInfo(feed, middleColWidth, infoBoxHeight)
		streamBox := m.renderLiveStream(feed, middleColWidth, streamHeight)
		aiBox := m.renderAIPanel(feed, aiColWidth, aiHeight)

		middleColumn := lipgloss.JoinVertical(lipgloss.Left, infoBox, streamBox)
		rightBuilder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, middleColumn, "  ", aiBox))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, "  ", rightBuilder.String())
}

func (m Model) renderFeedList(width, height int) string {
	visibleFeeds := height - 2
	if visibleFeeds < 3 {
		visibleFeeds = 3
	}

	feedStartIdx := 0
	feedEndIdx := len(m.feeds.Feeds)
	if len(m.feeds.Feeds) > visibleFeeds {
		halfVisible := visibleFeeds / 2
		feedStartIdx = m.feeds.SelectedIdx - halfVisible
		if feedStartIdx < 0 {
			feedStartIdx = 0
		}
		feedEndIdx = feedStartIdx + visibleFeeds
		if feedEndIdx > len(m.feeds.Feeds) {
			feedEndIdx = len(m.feeds.Feeds)
			feedStartIdx = feedEndIdx - visibleFeeds
			if feedStartIdx < 0 {
				feedStartIdx = 0
			}
		}
	}

	var builder strings.Builder
	if feedStartIdx > 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("  ^ more\n"))
	}

	for i := feedStartIdx; i < feedEndIdx; i++ {
		f := m.feeds.Feeds[i]
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.feeds.SelectedIdx {
			cursor = lipgloss.NewStyle().Foreground(ui.CyanColor).Render("> ")
			style = style.Foreground(ui.BrightCyanColor)
		}
		subscribed := ""
		if m.feeds.IsSubscribed(f.ID) {
			subscribed = " [ok]"
		}
		maxNameLen := width - 18
		if maxNameLen < 10 {
			maxNameLen = 10
		}
		feedName := ui.Truncate(f.Name, maxNameLen)
		category := ui.Truncate(f.Category, 8)
		line := fmt.Sprintf("%s%s [%s]%s", cursor, feedName, category, subscribed)
		builder.WriteString(style.Render(line))
		builder.WriteString("\n")
	}

	if feedEndIdx < len(m.feeds.Feeds) {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("  v more"))
	}

	return ui.RenderBoxWithTitle("My Feeds", builder.String(), width, height, ui.DarkCyanColor, ui.CyanColor)
}

func (m Model) renderInstructions(width, height int) string {
	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render("Navigation"))
	builder.WriteString("\n")
	builder.WriteString("  Up/Down  Select feed\n")
	builder.WriteString("  Tab      Next tab\n")
	builder.WriteString("  Shift+Tab Previous tab\n")
	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render("Actions"))
	builder.WriteString("\n")
	builder.WriteString("  s        Sub/Unsub\n")
	builder.WriteString("  e        Edit feed\n")
	builder.WriteString("  r        Reconnect to WS\n")
	builder.WriteString("  Shift+D  Delete my feed\n")
	builder.WriteString("  l        Logout\n")
	builder.WriteString("  q        Quit\n")
	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render("AI Analysis"))
	builder.WriteString("\n")
	builder.WriteString("  p        Edit prompt\n")
	builder.WriteString("  Enter    Send prompt\n")
	builder.WriteString("  Esc      Exit prompt\n")
	builder.WriteString("  m        Auto/Manual\n")

	return ui.RenderBoxWithTitle("Instructions", builder.String(), width, height, ui.DarkMagentaColor, ui.MagentaColor)
}

func (m Model) renderFeedInfo(feed api.Feed, width, height int) string {
	maxContentWidth := width - 6
	if maxContentWidth < 30 {
		maxContentWidth = 30
	}

	var builder strings.Builder
	builder.WriteString(ui.Truncate(feed.Name, maxContentWidth))
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("Category: %s\n", ui.Truncate(feed.Category, maxContentWidth-10)))
	builder.WriteString(fmt.Sprintf("URL: %s\n", ui.Truncate(feed.URL, maxContentWidth-5)))
	if feed.EventName != "" {
		builder.WriteString(fmt.Sprintf("Event: %s\n", ui.Truncate(feed.EventName, maxContentWidth-7)))
	}

	subStatus := "[-] Not Subscribed"
	if m.feeds.IsSubscribed(feed.ID) {
		subStatus = "[+] Subscribed"
	}
	builder.WriteString(fmt.Sprintf("Status: %s\n", subStatus))
	builder.WriteString(fmt.Sprintf("WS: %s", m.wsStatus))

	return ui.RenderBoxWithTitle("Feed Info", builder.String(), width, height, ui.DarkCyanColor, ui.CyanColor)
}

func (m Model) renderLiveStream(feed api.Feed, width, height int) string {
	maxDataWidth := width - 15
	if maxDataWidth < 20 {
		maxDataWidth = 20
	}

	var builder strings.Builder
	entries := m.feeds.Entries[feed.ID]
	if len(entries) == 0 {
		if m.wsStatus != "connected" {
			builder.WriteString("[!] WS not connected\n")
			builder.WriteString("Reconnecting...")
		} else if !m.feeds.IsSubscribed(feed.ID) {
			builder.WriteString("Press 's' to subscribe...")
		} else {
			builder.WriteString("[+] Connected & Subscribed\n")
			builder.WriteString("Waiting for data...")
		}
	} else {
		showCount := height - 3
		if len(entries) < showCount {
			showCount = len(entries)
		}
		for i := 0; i < showCount; i++ {
			e := entries[i]
			timestamp := e.Time.Format("15:04:05")
			builder.WriteString(fmt.Sprintf("%s %s\n", timestamp, ui.Truncate(e.Data, maxDataWidth)))
		}
	}

	return ui.RenderBoxWithTitle("Live Stream", builder.String(), width, height, ui.DarkCyanColor, ui.CyanColor)
}

func (m Model) renderAIPanel(feed api.Feed, width, height int) string {
	var builder strings.Builder

	// Mode and pause status
	modeLabel := "Manual"
	if m.ai.AutoMode {
		modeLabel = fmt.Sprintf("Auto (%ds)", m.ai.Interval)
	}
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Mode: "))
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.BrightCyanColor).Render(modeLabel))

	if m.ai.Paused[feed.ID] {
		builder.WriteString("  ")
		builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.RedColor).Render("|| PAUSED"))
	} else {
		builder.WriteString("  ")
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.GreenColor).Render("> Active"))
	}
	builder.WriteString("\n")

	separatorWidth := width - 8
	if separatorWidth < 20 {
		separatorWidth = 20
	}
	separator := strings.Repeat("-", separatorWidth)
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DarkMagentaColor).Render(separator))
	builder.WriteString("\n\n")

	// Output stream
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Output Stream (last 3):"))
	builder.WriteString("\n")

	outputAreaHeight := height - 16
	if outputAreaHeight < 6 {
		outputAreaHeight = 6
	}
	aiTextWidth := width - 10
	if aiTextWidth < 30 {
		aiTextWidth = 30
	}

	feedAIHistory := m.ai.OutputHistories[feed.ID]
	feedAIResponse := m.ai.Responses[feed.ID]
	feedAILoading := m.ai.Loading[feed.ID]

	if feedAILoading && len(feedAIHistory) == 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.MagentaColor).Render("[...] Querying LLM..."))
		builder.WriteString("\n")
	}

	if len(feedAIHistory) == 0 && !feedAILoading {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("No outputs yet. Press 'p' then Enter."))
		builder.WriteString("\n")
	} else {
		var outputContent strings.Builder
		maxOutputs := 3
		startIdx := 0
		if len(feedAIHistory) > maxOutputs {
			startIdx = len(feedAIHistory) - maxOutputs
		}

		for i := startIdx; i < len(feedAIHistory); i++ {
			entry := feedAIHistory[i]
			timestamp := entry.Timestamp.Format("15:04:05")
			header := fmt.Sprintf("[%s | %s | %dms]", timestamp, entry.Provider, entry.Duration)
			outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(header))
			outputContent.WriteString("\n")

			wrapped := ui.WrapText(entry.Response, aiTextWidth)
			outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.WhiteColor).Render(wrapped))
			outputContent.WriteString("\n")

			if i < len(feedAIHistory)-1 {
				outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.GrayColor).Render("---"))
				outputContent.WriteString("\n")
			}
		}

		if feedAILoading && feedAIResponse != "" {
			outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.GrayColor).Render("---"))
			outputContent.WriteString("\n")
			outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.MagentaColor).Render("[...] Streaming..."))
			outputContent.WriteString("\n")
			wrapped := ui.WrapText(feedAIResponse, aiTextWidth)
			outputContent.WriteString(lipgloss.NewStyle().Foreground(ui.WhiteColor).Render(wrapped))
			outputContent.WriteString("\n")
		}

		fullOutput := outputContent.String()
		lines := strings.Split(fullOutput, "\n")
		if len(lines) > outputAreaHeight {
			startIndex := len(lines) - outputAreaHeight
			if startIndex < 0 {
				startIndex = 0
			}
			lines = lines[startIndex:]
			fullOutput = strings.Join(lines, "\n")
		}
		builder.WriteString(fullOutput)
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DarkMagentaColor).Render(separator))
	builder.WriteString("\n")

	// Prompt input
	promptPrefix := lipgloss.NewStyle().Foreground(ui.GreenColor).Render("> ")
	builder.WriteString(promptPrefix)

	feedPrompt := m.ai.GetPrompt(feed.ID)
	promptWidth := width - 12
	if promptWidth < 20 {
		promptWidth = 20
	}
	feedPrompt.SetWidth(promptWidth)

	if m.ai.Focused {
		feedPrompt.Focus()
	} else {
		feedPrompt.Blur()
	}

	builder.WriteString(feedPrompt.View())
	builder.WriteString("\n\n")

	controlHint := "Enter: send | m: mode | p: edit | Shift+P: pause"
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(controlHint))

	return ui.RenderBoxWithTitle("AI Analysis", builder.String(), width, height, ui.DarkMagentaColor, ui.MagentaColor)
}

func (m Model) viewFeedDetail() string {
	if m.feeds.SelectedFeed == nil {
		return ui.ContentStyle.Render("Select a feed to view details.")
	}
	feed := m.feeds.SelectedFeed
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Category: %s | Owner: %s\n", feed.Category, feed.OwnerName))
	builder.WriteString(fmt.Sprintf("URL: %s\n", ui.Truncate(feed.URL, 80)))
	builder.WriteString(fmt.Sprintf("Event: %s\n", feed.EventName))
	builder.WriteString(fmt.Sprintf("Public: %v | Active: %v\n", feed.IsPublic, feed.IsActive))

	subStatus := lipgloss.NewStyle().Foreground(ui.RedColor).Render("not subscribed")
	if m.feeds.IsSubscribed(feed.ID) {
		subStatus = lipgloss.NewStyle().Foreground(ui.GreenColor).Render("subscribed [ok]")
	}
	builder.WriteString(fmt.Sprintf("Status: %s | WS: %s\n", subStatus, m.wsStatus))

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.DimCyanColor).Render("Live data (latest first):"))
	builder.WriteString("\n")

	availableHeight := m.ui.TermHeight - 20
	if availableHeight < 5 {
		availableHeight = 5
	}

	entries := m.feeds.Entries[feed.ID]
	if len(entries) == 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("No data yet. Subscribe (s) or wait for updates."))
	} else {
		showCount := availableHeight
		if len(entries) < showCount {
			showCount = len(entries)
		}
		for i := 0; i < showCount; i++ {
			e := entries[i]
			builder.WriteString(fmt.Sprintf("[%s] %s\n", e.Time.Format("15:04:05"), ui.Truncate(e.Data, 100)))
		}
		if len(entries) > showCount {
			builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(fmt.Sprintf("  ... and %d more entries", len(entries)-showCount)))
		}
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("s: subscribe/unsubscribe | Esc: go back to My Feeds"))

	boxWidth := m.ui.TermWidth - 4
	if boxWidth > 120 {
		boxWidth = 120
	}
	boxHeight := m.ui.TermHeight - 10
	if boxHeight < 15 {
		boxHeight = 15
	}

	return ui.RenderBoxWithTitle(feed.Name, builder.String(), boxWidth, boxHeight, ui.DarkCyanColor, ui.CyanColor)
}

func (m Model) viewRegisterFeed() string {
	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("Register New WebSocket Feed"))
	builder.WriteString("\n\n")

	labels := m.feedForm.Labels()
	inputs := m.feedForm.Inputs()

	for i, label := range labels {
		labelStyle := lipgloss.NewStyle().Foreground(ui.DimCyanColor)
		if i == m.feedForm.FocusIndex {
			labelStyle = lipgloss.NewStyle().Foreground(ui.CyanColor).Bold(true)
		}
		builder.WriteString(labelStyle.Render(label + ": "))
		builder.WriteString(inputs[i].View())
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Up/Down navigate | Enter submit | Esc cancel | * required"))

	if m.ui.Loading {
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("%s Creating feed...", m.spinner.View()))
	}
	if m.messages.Error != "" {
		builder.WriteString("\n")
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.RedColor).Render(m.messages.Error))
	}

	return ui.ContentStyle.Render(builder.String())
}

func (m Model) viewEditFeed() string {
	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("Edit Feed"))
	builder.WriteString("\n\n")

	labels := m.feedForm.Labels()
	inputs := m.feedForm.Inputs()

	for i, label := range labels {
		labelStyle := lipgloss.NewStyle().Foreground(ui.DimCyanColor)
		if i == m.feedForm.FocusIndex {
			labelStyle = lipgloss.NewStyle().Foreground(ui.CyanColor).Bold(true)
		}
		builder.WriteString(labelStyle.Render(label + ": "))
		builder.WriteString(inputs[i].View())
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Up/Down navigate | Enter save | Esc cancel | * required"))

	if m.ui.Loading {
		builder.WriteString("\n")
		builder.WriteString(fmt.Sprintf("%s Updating feed...", m.spinner.View()))
	}
	if m.messages.Error != "" {
		builder.WriteString("\n")
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.RedColor).Render(m.messages.Error))
	}

	return ui.ContentStyle.Render(builder.String())
}

func (m Model) viewAPI() string {
	var builder strings.Builder
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("API & Integration"))
	builder.WriteString("\n\n")
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("Use these Feed IDs to subscribe via WebSocket or API."))
	builder.WriteString("\n\n")

	if len(m.feeds.Feeds) == 0 {
		builder.WriteString("No feeds available. Register a feed first.")
	} else {
		builder.WriteString(lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%-30s %-40s", "Feed Name", "Feed ID")))
		builder.WriteString("\n")
		builder.WriteString(strings.Repeat("-", 70))
		builder.WriteString("\n")

		for _, f := range m.feeds.Feeds {
			builder.WriteString(fmt.Sprintf("%-30s %-40s\n", ui.Truncate(f.Name, 28), f.ID))
		}
	}

	builder.WriteString("\n\n")
	builder.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ui.CyanColor).Render("WebSocket Subscription"))
	builder.WriteString("\n")
	builder.WriteString("Connect to: " + m.backendURL + "/ws")
	builder.WriteString("\n")
	builder.WriteString("Event: 'subscribe-feed' Payload: { \"feedId\": \"<FEED_ID>\" }")
	builder.WriteString("\n")
	builder.WriteString("Listen for: 'llm-broadcast' event for AI updates.")

	return ui.ContentStyle.Render(builder.String())
}

func (m Model) viewHelp() string {
	helpPages := getHelpPages()

	helpPage := m.ui.HelpPage
	if helpPage < 0 {
		helpPage = 0
	}
	if helpPage >= len(helpPages) {
		helpPage = len(helpPages) - 1
	}

	currentPage := helpPages[helpPage]

	var builder strings.Builder

	navStyle := lipgloss.NewStyle().Foreground(ui.DimCyanColor)
	pageIndicator := fmt.Sprintf("Page %d of %d", helpPage+1, len(helpPages))

	dots := ""
	for i := 0; i < len(helpPages); i++ {
		if i == helpPage {
			dots += lipgloss.NewStyle().Foreground(ui.CyanColor).Render(" * ")
		} else {
			dots += lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(" o ")
		}
	}

	builder.WriteString(navStyle.Render(pageIndicator))
	builder.WriteString("  ")
	builder.WriteString(dots)
	builder.WriteString("\n\n")

	contentLines := strings.Split(currentPage.content, "\n")

	startLine := m.ui.HelpScroll
	if startLine >= len(contentLines) {
		startLine = len(contentLines) - 1
	}
	if startLine < 0 {
		startLine = 0
	}

	visibleLines := m.ui.TermHeight - 16
	if visibleLines < 10 {
		visibleLines = 10
	}

	endLine := startLine + visibleLines
	if endLine > len(contentLines) {
		endLine = len(contentLines)
	}

	if startLine > 0 {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("  ^ scroll up for more"))
		builder.WriteString("\n")
	}

	for _, line := range contentLines[startLine:endLine] {
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
			builder.WriteString(lipgloss.NewStyle().Foreground(ui.DarkCyanColor).Render(line))
		} else if len(line) > 0 && line[0] != ' ' && strings.HasSuffix(strings.TrimSpace(line), ":") {
			builder.WriteString(lipgloss.NewStyle().Foreground(ui.CyanColor).Bold(true).Render(line))
		} else if strings.HasPrefix(strings.TrimSpace(line), "-") || strings.HasPrefix(strings.TrimSpace(line), "*") {
			builder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA")).Render(line))
		} else {
			builder.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#DDDDDD")).Render(line))
		}
		builder.WriteString("\n")
	}

	if endLine < len(contentLines) {
		builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render("  v scroll down for more"))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	navHint := "<- -> navigate pages | Tab switch tabs | q quit"
	builder.WriteString(lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(navHint))

	boxWidth := m.ui.TermWidth - 4
	if boxWidth > 100 {
		boxWidth = 100
	}
	boxHeight := m.ui.TermHeight - 10
	if boxHeight < 20 {
		boxHeight = 20
	}

	return ui.RenderBoxWithTitle(currentPage.title, builder.String(), boxWidth, boxHeight, ui.DarkCyanColor, ui.CyanColor)
}

func (m Model) viewFooter() string {
	if m.messages.Error != "" {
		return lipgloss.NewStyle().Foreground(ui.RedColor).Render(m.messages.Error)
	}
	if m.messages.Status != "" {
		return lipgloss.NewStyle().Foreground(ui.DimCyanColor).Render(m.messages.Status)
	}
	return ""
}

// Helper methods

func (m Model) userAgent() string {
	return "TurboStream TUI"
}

func (m Model) nextWSListen() tea.Cmd {
	if m.wsClient == nil {
		return nil
	}
	return m.wsClient.ListenCmd()
}

func (m Model) sendAIQuery() tea.Cmd {
	if m.wsClient == nil || m.feeds.SelectedFeed == nil {
		return func() tea.Msg {
			return model.AIResponseMsg{RequestID: m.ai.RequestID, Err: fmt.Errorf("not connected or no feed selected")}
		}
	}
	return m.sendAIQueryForFeed(m.feeds.SelectedFeed.ID, m.ai.RequestID)
}

func (m Model) sendAIQueryForFeed(feedID, requestID string) tea.Cmd {
	if m.wsClient == nil {
		return func() tea.Msg {
			return model.AIResponseMsg{RequestID: requestID, Err: fmt.Errorf("not connected")}
		}
	}

	if m.ai.Paused[feedID] {
		return nil
	}

	prompt := ""
	if feedPrompt, ok := m.ai.Prompts[feedID]; ok {
		prompt = feedPrompt.Value()
	}

	if prompt == "" {
		return nil
	}

	systemPrompt := ""
	for _, f := range m.feeds.Feeds {
		if f.ID == feedID {
			systemPrompt = f.SystemPrompt
			break
		}
	}

	wsClient := m.wsClient

	return func() tea.Msg {
		err := wsClient.SendLLMQuery(feedID, prompt, systemPrompt, requestID)
		if err != nil {
			return model.AIResponseMsg{RequestID: requestID, Err: err}
		}
		return nil
	}
}

func getenvDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Help page content

type helpPage struct {
	title   string
	content string
}

func getHelpPages() []helpPage {
	return []helpPage{
		{
			title: "Getting Started",
			content: `Welcome to TurboStream TUI!

TurboStream is a real-time data streaming platform that allows you to
subscribe to WebSocket feeds and get AI-powered analysis of incoming data.

NAVIGATION
----------
  Tab / Shift+Tab    Switch between tabs
  Arrow Keys         Navigate within views
  Enter              Select/Confirm
  Esc                Go back/Cancel
  q                  Quit application

TABS OVERVIEW
-------------
  Dashboard       View your subscribed feeds in real-time
  Register Feed   Create and register new WebSocket feeds
  My Feeds        Manage your registered feeds
  Help            You are here! Documentation and guides

Use <- and -> arrow keys to navigate between help pages.`,
		},
		{
			title: "Dashboard",
			content: `DASHBOARD VIEW
==============

The Dashboard is your main view for monitoring real-time data streams.

LAYOUT
------
  Left Panel      List of your subscribed feeds
  Middle Panel    Live stream data from selected feed
  Right Panel     AI-powered analysis of the data

KEYBOARD SHORTCUTS
------------------
  Up/Down         Select different feed in sidebar

The Dashboard displays real-time streaming data from your subscribed feeds.`,
		},
		{
			title: "Register Feed",
			content: `REGISTERING A NEW FEED
======================

Create custom WebSocket feeds to stream data from any source.

REQUIRED FIELDS
---------------
  Feed Name *       A unique name for your feed
  WebSocket URL *   The WebSocket endpoint URL (wss:// or ws://)

OPTIONAL FIELDS
---------------
  Description       Brief description of what the feed provides
  Category          Category for organization (crypto, stocks, etc.)
  Event Name        Socket.io event name (if applicable)
  Subscription Msg  JSON message to send after connecting
  AI System Prompt  Custom prompt for AI analysis of this feed

TIPS
----
  - Test your WebSocket URL before registering
  - Use descriptive names for easy identification
  - Set a good AI prompt for better analysis`,
		},
		{
			title: "My Feeds",
			content: `MANAGING YOUR FEEDS
===================

The My Feeds view shows all your registered feeds and their status,
with live data streaming and AI-powered analysis.

KEYBOARD SHORTCUTS
------------------
  Up/Down     Navigate feed list
  Enter       View feed details
  s           Subscribe/Unsubscribe to feed
  D           Delete selected feed (Shift+D)
  r           Reconnect WebSocket
  p           Open custom AI prompt input (per-feed)
  Shift+P     Pause/Resume AI Analysis
  Esc         Return from feed details

AI ANALYSIS
-----------
The AI panel provides intelligent insights about your data streams.
Press 'p' to enter a custom prompt for analysis.
Press 'Shift+P' to pause/resume AI queries for current feed.`,
		},
		{
			title: "API & WebSockets",
			content: `API & WEBSOCKET INTEGRATION
===========================

TurboStream allows external applications to consume feed data and AI
analysis in real-time via WebSockets.

STEP 1: CONNECT
---------------
  URL: ws://localhost:7210/ws

STEP 2: SUBSCRIBE
-----------------
  LLM Output Only:
  {
    "type": "subscribe-llm",
    "payload": { "feedId": "<YOUR_FEED_ID>" }
  }

  Raw Feed Data Only:
  {
    "type": "subscribe-feed",
    "payload": { "feedId": "<YOUR_FEED_ID>" }
  }

Use the "API" tab to find your Feed IDs.`,
		},
		{
			title: "Tips & Tricks",
			content: `TIPS & TRICKS
=============

PERFORMANCE
-----------
  - Limit active subscriptions for better performance
  - Use specific AI prompts for more relevant analysis
  - Close unused feeds to reduce bandwidth

TROUBLESHOOTING
---------------
  Connection Issues:
  - Check your internet connection
  - Verify WebSocket URL is correct
  - Some feeds require authentication

  No Data Showing:
  - Ensure you're subscribed to the feed
  - Check if the feed requires a subscription message
  - Try reconnecting with 'r'

KEYBOARD REFERENCE
------------------
  Global:
    Tab/Shift+Tab   Switch tabs
    q               Quit

  Dashboard & My Feeds:
    Up/Down         Navigate feed list
    i               Change AI interval
    m               Toggle AI auto/manual
    p               Custom AI prompt (per-feed)
    Shift+P         Pause/Resume AI
    r               Reconnect WebSocket`,
		},
	}
}
