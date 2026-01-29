// Package metrics provides observability metrics collection and computation.
package metrics

import (
	"sort"
	"sync"
	"time"
)

// FeedMetrics contains observability metrics for a single feed
type FeedMetrics struct {
	// Metadata
	FeedID      string
	Name        string
	LastUpdated time.Time

	// Stream / WebSocket health
	MessagesReceivedTotal uint64
	MessagesPerSecond10s  float64
	BytesReceivedTotal    uint64
	BytesPerSecond10s     float64
	LastMessageAgeSeconds float64
	WSConnected           bool
	ReconnectsTotal       uint64
	CurrentUptimeSeconds  float64

	// In-memory cache health
	CacheItemsCurrent    int
	CacheApproxBytes     uint64
	OldestItemAgeSeconds float64

	// Packet loss / context overflow
	MessagesDroppedTotal  uint64
	ContextEvictionsTotal uint64
	DropRatePercent       float64

	// Payload size stats
	PayloadSizeLastBytes int
	PayloadSizeAvgBytes  float64
	PayloadSizeMaxBytes  int

	// LLM / token usage
	LLMRequestsTotal          uint64
	InputTokensTotal          uint64
	OutputTokensTotal         uint64
	InputTokensLast           int
	OutputTokensLast          int
	ContextUtilizationPercent float64
	LLMErrorsTotal            uint64
	EventsInContextCurrent    int
	TTFTMs                    float64
	TTFTAvgMs                 float64
	GenerationTimeMs          float64
	GenerationTimeAvgMs       float64

	// History for sparkline charts
	MsgRateHistory     []float64
	CacheBytesHistory  []float64
	GenTimeHistory     []float64
	PayloadSizeHistory []float64
}

// DashboardMetrics holds metrics for all feeds
type DashboardMetrics struct {
	Feeds       []FeedMetrics
	SelectedIdx int
}

// slidingWindow tracks values over time for rate calculations
type slidingWindow struct {
	mu       sync.Mutex
	samples  []windowSample
	duration time.Duration
}

type windowSample struct {
	timestamp time.Time
	value     float64
}

func newSlidingWindow(duration time.Duration) *slidingWindow {
	return &slidingWindow{
		samples:  make([]windowSample, 0, 1000),
		duration: duration,
	}
}

func (w *slidingWindow) Add(value float64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.samples = append(w.samples, windowSample{timestamp: now, value: value})
	w.prune(now)
}

func (w *slidingWindow) prune(now time.Time) {
	cutoff := now.Add(-w.duration)
	idx := 0
	for i, s := range w.samples {
		if s.timestamp.After(cutoff) {
			idx = i
			break
		}
	}
	if idx > 0 {
		w.samples = w.samples[idx:]
	}
}

func (w *slidingWindow) Rate(windowDuration time.Duration) float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	w.prune(now)

	cutoff := now.Add(-windowDuration)
	var sum float64
	for _, s := range w.samples {
		if s.timestamp.After(cutoff) {
			sum += s.value
		}
	}
	return sum / windowDuration.Seconds()
}

// payloadSampler tracks payload sizes for statistics
type payloadSampler struct {
	mu       sync.Mutex
	samples  []int
	times    []time.Time
	maxSize  int
	duration time.Duration
}

func newPayloadSampler(maxSamples int, duration time.Duration) *payloadSampler {
	return &payloadSampler{
		samples:  make([]int, 0, maxSamples),
		times:    make([]time.Time, 0, maxSamples),
		maxSize:  maxSamples,
		duration: duration,
	}
}

func (p *payloadSampler) Add(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()

	// Prune old samples
	cutoff := now.Add(-p.duration)
	idx := 0
	for i, t := range p.times {
		if t.After(cutoff) {
			idx = i
			break
		}
	}
	if idx > 0 {
		p.samples = p.samples[idx:]
		p.times = p.times[idx:]
	}

	p.samples = append(p.samples, size)
	p.times = append(p.times, now)

	if len(p.samples) > p.maxSize {
		p.samples = p.samples[1:]
		p.times = p.times[1:]
	}
}

func (p *payloadSampler) Stats() (avg float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.samples) == 0 {
		return 0
	}

	var sum int
	for _, s := range p.samples {
		sum += s
	}
	return float64(sum) / float64(len(p.samples))
}

// tokenSampler tracks LLM token usage
type tokenSampler struct {
	mu                sync.Mutex
	ttfts             []float64
	genTimes          []float64
	times             []time.Time
	maxSize           int
	duration          time.Duration
	totalInputTokens  uint64
	totalOutputTokens uint64
	lastInputTokens   int
	lastOutputTokens  int
	lastTTFT          float64
	lastGenTime       float64
}

func newTokenSampler(maxSamples int, duration time.Duration) *tokenSampler {
	return &tokenSampler{
		ttfts:    make([]float64, 0, maxSamples),
		genTimes: make([]float64, 0, maxSamples),
		times:    make([]time.Time, 0, maxSamples),
		maxSize:  maxSamples,
		duration: duration,
	}
}

func (t *tokenSampler) Add(promptTokens, responseTokens int, ttftMs, genTimeMs float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()

	if promptTokens > 0 {
		t.totalInputTokens += uint64(promptTokens)
	}
	if responseTokens > 0 {
		t.totalOutputTokens += uint64(responseTokens)
	}
	t.lastInputTokens = promptTokens
	t.lastOutputTokens = responseTokens
	t.lastTTFT = ttftMs
	t.lastGenTime = genTimeMs

	// Prune old samples
	cutoff := now.Add(-t.duration)
	idx := 0
	for i, tm := range t.times {
		if tm.After(cutoff) {
			idx = i
			break
		}
	}
	if idx > 0 {
		t.ttfts = t.ttfts[idx:]
		t.genTimes = t.genTimes[idx:]
		t.times = t.times[idx:]
	}

	t.ttfts = append(t.ttfts, ttftMs)
	t.genTimes = append(t.genTimes, genTimeMs)
	t.times = append(t.times, now)
}

func (t *tokenSampler) Stats() (inputTotal, outputTotal uint64, inputLast, outputLast int, ttftLast, ttftAvg, genTimeLast, genTimeAvg float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	inputTotal = t.totalInputTokens
	outputTotal = t.totalOutputTokens
	inputLast = t.lastInputTokens
	outputLast = t.lastOutputTokens
	ttftLast = t.lastTTFT
	genTimeLast = t.lastGenTime

	if len(t.ttfts) == 0 {
		return
	}

	var ttftSum float64
	for _, v := range t.ttfts {
		ttftSum += v
	}
	ttftAvg = ttftSum / float64(len(t.ttfts))

	if len(t.genTimes) > 0 {
		var genSum float64
		for _, v := range t.genTimes {
			genSum += v
		}
		genTimeAvg = genSum / float64(len(t.genTimes))
	}

	return
}

// historySampler keeps a fixed-size ring buffer for sparklines
type historySampler struct {
	mu      sync.Mutex
	samples []float64
	maxSize int
}

func newHistorySampler(maxSamples int) *historySampler {
	return &historySampler{
		samples: make([]float64, 0, maxSamples),
		maxSize: maxSamples,
	}
}

func (h *historySampler) Add(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = append(h.samples, value)
	if len(h.samples) > h.maxSize {
		h.samples = h.samples[1:]
	}
}

func (h *historySampler) Values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := make([]float64, len(h.samples))
	copy(result, h.samples)
	return result
}

// Collector collects and computes metrics from feed data
type Collector struct {
	mu              sync.RWMutex
	feedMetrics     map[string]*FeedMetrics
	messageWindows  map[string]*slidingWindow
	byteWindows     map[string]*slidingWindow
	payloadSamples  map[string]*payloadSampler
	llmTokenSamples map[string]*tokenSampler
	startTimes      map[string]time.Time
	lastMsgTimes    map[string]time.Time
	msgRateHistory  map[string]*historySampler
	cacheBytesHist  map[string]*historySampler
	genTimeHistory  map[string]*historySampler
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		feedMetrics:     make(map[string]*FeedMetrics),
		messageWindows:  make(map[string]*slidingWindow),
		byteWindows:     make(map[string]*slidingWindow),
		payloadSamples:  make(map[string]*payloadSampler),
		llmTokenSamples: make(map[string]*tokenSampler),
		startTimes:      make(map[string]time.Time),
		lastMsgTimes:    make(map[string]time.Time),
		msgRateHistory:  make(map[string]*historySampler),
		cacheBytesHist:  make(map[string]*historySampler),
		genTimeHistory:  make(map[string]*historySampler),
	}
}

// InitFeed initializes metrics for a feed
func (c *Collector) InitFeed(feedID, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.feedMetrics[feedID]; !exists {
		c.feedMetrics[feedID] = &FeedMetrics{
			FeedID:      feedID,
			Name:        name,
			LastUpdated: time.Now(),
		}
		c.messageWindows[feedID] = newSlidingWindow(time.Minute)
		c.byteWindows[feedID] = newSlidingWindow(time.Minute)
		c.payloadSamples[feedID] = newPayloadSampler(1000, 5*time.Minute)
		c.llmTokenSamples[feedID] = newTokenSampler(100, 5*time.Minute)
		c.startTimes[feedID] = time.Now()
		c.msgRateHistory[feedID] = newHistorySampler(30)
		c.cacheBytesHist[feedID] = newHistorySampler(30)
		c.genTimeHistory[feedID] = newHistorySampler(30)
	}
}

// RecordMessage records a received message for a feed
func (c *Collector) RecordMessage(feedID string, payloadSize int) {
	c.mu.Lock()
	fm, exists := c.feedMetrics[feedID]
	if !exists {
		c.mu.Unlock()
		return
	}

	fm.MessagesReceivedTotal++
	if payloadSize > 0 {
		fm.BytesReceivedTotal += uint64(payloadSize)
	}
	fm.PayloadSizeLastBytes = payloadSize
	if payloadSize > fm.PayloadSizeMaxBytes {
		fm.PayloadSizeMaxBytes = payloadSize
	}
	fm.LastUpdated = time.Now()
	c.lastMsgTimes[feedID] = time.Now()

	msgWindow := c.messageWindows[feedID]
	byteWindow := c.byteWindows[feedID]
	sampler := c.payloadSamples[feedID]
	c.mu.Unlock()

	msgWindow.Add(1)
	byteWindow.Add(float64(payloadSize))
	sampler.Add(payloadSize)
}

// RecordWSStatus records WebSocket connection status
func (c *Collector) RecordWSStatus(feedID string, connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fm, exists := c.feedMetrics[feedID]
	if !exists {
		return
	}

	wasConnected := fm.WSConnected
	fm.WSConnected = connected

	if !connected && wasConnected {
		fm.ReconnectsTotal++
		c.startTimes[feedID] = time.Now()
	} else if connected && !wasConnected {
		c.startTimes[feedID] = time.Now()
	}
}

// RecordCacheStats records cache statistics
func (c *Collector) RecordCacheStats(feedID string, itemCount int, approxBytes uint64, oldestAge float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fm, exists := c.feedMetrics[feedID]; exists {
		fm.CacheItemsCurrent = itemCount
		fm.CacheApproxBytes = approxBytes
		fm.OldestItemAgeSeconds = oldestAge
	}
}

// RecordPacketLoss records when a message is dropped
func (c *Collector) RecordPacketLoss(feedID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fm, exists := c.feedMetrics[feedID]; exists {
		fm.MessagesDroppedTotal++
		if fm.MessagesReceivedTotal > 0 {
			fm.DropRatePercent = float64(fm.MessagesDroppedTotal) / float64(fm.MessagesReceivedTotal) * 100
		}
	}
}

// RecordContextEviction records when older messages are evicted
func (c *Collector) RecordContextEviction(feedID string, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if fm, exists := c.feedMetrics[feedID]; exists && count > 0 {
		fm.ContextEvictionsTotal += uint64(count)
	}
}

// RecordLLMRequest records an LLM request with token counts and timing
func (c *Collector) RecordLLMRequest(feedID string, inputTokens, outputTokens int, ttftMs, genTimeMs float64, eventsInContext int, isError bool) {
	c.mu.Lock()
	fm, exists := c.feedMetrics[feedID]
	if !exists {
		c.mu.Unlock()
		return
	}

	fm.LLMRequestsTotal++
	fm.EventsInContextCurrent = eventsInContext
	if isError {
		fm.LLMErrorsTotal++
	}

	sampler := c.llmTokenSamples[feedID]
	c.mu.Unlock()

	sampler.Add(inputTokens, outputTokens, ttftMs, genTimeMs)
}

// GetMetrics returns computed metrics for all feeds
func (c *Collector) GetMetrics() DashboardMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var feeds []FeedMetrics

	for feedID, fm := range c.feedMetrics {
		metrics := *fm

		// Compute rates
		if msgWindow, ok := c.messageWindows[feedID]; ok {
			metrics.MessagesPerSecond10s = msgWindow.Rate(10 * time.Second)
		}
		if byteWindow, ok := c.byteWindows[feedID]; ok {
			metrics.BytesPerSecond10s = byteWindow.Rate(10 * time.Second)
		}

		// Compute payload stats
		if sampler, ok := c.payloadSamples[feedID]; ok {
			metrics.PayloadSizeAvgBytes = sampler.Stats()
		}

		// Compute LLM stats
		if sampler, ok := c.llmTokenSamples[feedID]; ok {
			inputTotal, outputTotal, inputLast, outputLast, ttftLast, ttftAvg, genTimeLast, genTimeAvg := sampler.Stats()
			metrics.InputTokensTotal = inputTotal
			metrics.OutputTokensTotal = outputTotal
			metrics.InputTokensLast = inputLast
			metrics.OutputTokensLast = outputLast
			metrics.TTFTMs = ttftLast
			metrics.TTFTAvgMs = ttftAvg
			metrics.GenerationTimeMs = genTimeLast
			metrics.GenerationTimeAvgMs = genTimeAvg

			const modelContextLimit = 128000
			if inputLast > 0 {
				metrics.ContextUtilizationPercent = (float64(inputLast) / modelContextLimit) * 100
			}
		}

		// Compute uptime and last message age
		if startTime, ok := c.startTimes[feedID]; ok {
			metrics.CurrentUptimeSeconds = now.Sub(startTime).Seconds()
		}
		if lastMsg, ok := c.lastMsgTimes[feedID]; ok {
			metrics.LastMessageAgeSeconds = now.Sub(lastMsg).Seconds()
		}

		// Sample history for sparklines
		if sampler, ok := c.msgRateHistory[feedID]; ok {
			sampler.Add(metrics.MessagesPerSecond10s)
			metrics.MsgRateHistory = sampler.Values()
		}
		if sampler, ok := c.cacheBytesHist[feedID]; ok {
			sampler.Add(float64(metrics.CacheApproxBytes))
			metrics.CacheBytesHistory = sampler.Values()
		}
		if sampler, ok := c.genTimeHistory[feedID]; ok {
			sampler.Add(metrics.GenerationTimeMs)
			metrics.GenTimeHistory = sampler.Values()
		}

		feeds = append(feeds, metrics)
	}

	// Sort by name for consistent ordering
	sort.Slice(feeds, func(i, j int) bool {
		return feeds[i].Name < feeds[j].Name
	})

	return DashboardMetrics{Feeds: feeds, SelectedIdx: 0}
}

// GetFeedMetrics returns metrics for a specific feed
func (c *Collector) GetFeedMetrics(feedID string) *FeedMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fm, exists := c.feedMetrics[feedID]
	if !exists {
		return nil
	}

	metrics := *fm
	now := time.Now()

	if msgWindow, ok := c.messageWindows[feedID]; ok {
		metrics.MessagesPerSecond10s = msgWindow.Rate(10 * time.Second)
	}
	if byteWindow, ok := c.byteWindows[feedID]; ok {
		metrics.BytesPerSecond10s = byteWindow.Rate(10 * time.Second)
	}
	if sampler, ok := c.payloadSamples[feedID]; ok {
		metrics.PayloadSizeAvgBytes = sampler.Stats()
	}
	if startTime, ok := c.startTimes[feedID]; ok {
		metrics.CurrentUptimeSeconds = now.Sub(startTime).Seconds()
	}
	if lastMsg, ok := c.lastMsgTimes[feedID]; ok {
		metrics.LastMessageAgeSeconds = now.Sub(lastMsg).Seconds()
	}

	return &metrics
}
