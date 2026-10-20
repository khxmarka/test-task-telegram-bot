package agent

import "sync"

const maxHistory = 100

type HistoryStore struct {
	mu     sync.Mutex
	byChat map[int64][]Message
}

func NewHistoryStore() *HistoryStore {
	return &HistoryStore{byChat: map[int64][]Message{}}
}

// Recent returns the last n user/assistant turns. Tool protocol is never stored,
// so the next Groq call cannot see a dangling assistant tool_calls message.
func (h *HistoryStore) Recent(chatID int64, n int) []Message {
	if n <= 0 {
		n = 12
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	msgs := h.byChat[chatID]
	if len(msgs) > n {
		msgs = msgs[len(msgs)-n:]
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

func (h *HistoryStore) Append(chatID int64, msgs ...Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cur := append(h.byChat[chatID], msgs...)
	if len(cur) > maxHistory {
		cur = cur[len(cur)-maxHistory:]
	}
	h.byChat[chatID] = cur
}
