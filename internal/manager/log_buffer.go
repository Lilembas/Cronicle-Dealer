package manager

import (
	"sync"

	"go.uber.org/zap/zapcore"
)

type LogEntry struct {
	Level   string `json:"level"`
	Time    string `json:"time"`
	Message string `json:"message"`
	Fields  string `json:"fields,omitempty"`
}

type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	maxSize int
	clients map[chan LogEntry]struct{}
}

func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
		clients: make(map[chan LogEntry]struct{}),
	}
}

func (lb *LogBuffer) Write(entry zapcore.Entry, fields string) {
	le := LogEntry{
		Level:   entry.Level.String(),
		Time:    entry.Time.Format("2006-01-02 15:04:05.000"),
		Message: entry.Message,
		Fields:  fields,
	}

	lb.mu.Lock()
	if len(lb.entries) >= lb.maxSize {
		lb.entries = lb.entries[len(lb.entries)/2:]
	}
	lb.entries = append(lb.entries, le)

	for ch := range lb.clients {
		select {
		case ch <- le:
		default:
					}
	}
	lb.mu.Unlock()
}

func (lb *LogBuffer) GetEntries() []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	result := make([]LogEntry, len(lb.entries))
	copy(result, lb.entries)
	return result
}

func (lb *LogBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 256)
	lb.mu.Lock()
	lb.clients[ch] = struct{}{}
	lb.mu.Unlock()
	return ch
}

func (lb *LogBuffer) Unsubscribe(ch chan LogEntry) {
	lb.mu.Lock()
	delete(lb.clients, ch)
	close(ch)
	lb.mu.Unlock()
}
