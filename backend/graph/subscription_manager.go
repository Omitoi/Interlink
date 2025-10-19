package graph

import (
	"sync"
	"time"

	"gitea.kood.tech/petrkubec/match-me/backend/graph/model"
)

// SubscriptionManager handles GraphQL subscriptions with WebSocket integration
type SubscriptionManager struct {
	// Message subscriptions: chatID -> subscribers
	messageSubscribers map[string]map[chan *model.ChatMessage]bool
	messageMutex       sync.RWMutex

	// Connection subscriptions: userID -> subscribers
	connectionSubscribers map[string]map[chan *model.Connection]bool
	connectionMutex       sync.RWMutex

	// Presence subscriptions: userID -> subscribers
	presenceSubscribers map[string]map[chan *model.PresenceUpdate]bool
	presenceMutex       sync.RWMutex

	// Typing subscriptions: chatID -> subscribers
	typingSubscribers map[string]map[chan *model.TypingStatus]bool
	typingMutex       sync.RWMutex
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		messageSubscribers:    make(map[string]map[chan *model.ChatMessage]bool),
		connectionSubscribers: make(map[string]map[chan *model.Connection]bool),
		presenceSubscribers:   make(map[string]map[chan *model.PresenceUpdate]bool),
		typingSubscribers:     make(map[string]map[chan *model.TypingStatus]bool),
	}
}

// Global subscription manager instance
var globalSubscriptionManager = NewSubscriptionManager()

// GetSubscriptionManager returns the global subscription manager
func GetSubscriptionManager() *SubscriptionManager {
	return globalSubscriptionManager
}

// Message Subscription Methods

// SubscribeToMessages subscribes to messages for a specific chat
func (sm *SubscriptionManager) SubscribeToMessages(chatID string) (<-chan *model.ChatMessage, func()) {
	sm.messageMutex.Lock()
	defer sm.messageMutex.Unlock()

	ch := make(chan *model.ChatMessage, 10) // Buffered channel to prevent blocking

	if sm.messageSubscribers[chatID] == nil {
		sm.messageSubscribers[chatID] = make(map[chan *model.ChatMessage]bool)
	}
	sm.messageSubscribers[chatID][ch] = true

	// Return channel and cleanup function
	cleanup := func() {
		sm.UnsubscribeFromMessages(chatID, ch)
	}

	return ch, cleanup
}

// UnsubscribeFromMessages removes a subscription for messages
func (sm *SubscriptionManager) UnsubscribeFromMessages(chatID string, ch chan *model.ChatMessage) {
	sm.messageMutex.Lock()
	defer sm.messageMutex.Unlock()

	if subscribers, ok := sm.messageSubscribers[chatID]; ok {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(sm.messageSubscribers, chatID)
		}
	}
	close(ch)
}

// BroadcastMessage sends a message to all subscribers of a chat
func (sm *SubscriptionManager) BroadcastMessage(message *model.ChatMessage) {
	sm.messageMutex.RLock()
	defer sm.messageMutex.RUnlock()

	if subscribers, ok := sm.messageSubscribers[message.ChatID]; ok {
		for ch := range subscribers {
			select {
			case ch <- message:
				// Message sent successfully
			default:
				// Channel is full, skip this subscriber
			}
		}
	}
}

// Connection Subscription Methods

// SubscribeToConnections subscribes to connection updates for a user
func (sm *SubscriptionManager) SubscribeToConnections(userID string) (<-chan *model.Connection, func()) {
	sm.connectionMutex.Lock()
	defer sm.connectionMutex.Unlock()

	ch := make(chan *model.Connection, 10) // Buffered channel

	if sm.connectionSubscribers[userID] == nil {
		sm.connectionSubscribers[userID] = make(map[chan *model.Connection]bool)
	}
	sm.connectionSubscribers[userID][ch] = true

	// Return channel and cleanup function
	cleanup := func() {
		sm.UnsubscribeFromConnections(userID, ch)
	}

	return ch, cleanup
}

// UnsubscribeFromConnections removes a subscription for connection updates
func (sm *SubscriptionManager) UnsubscribeFromConnections(userID string, ch chan *model.Connection) {
	sm.connectionMutex.Lock()
	defer sm.connectionMutex.Unlock()

	if subscribers, ok := sm.connectionSubscribers[userID]; ok {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(sm.connectionSubscribers, userID)
		}
	}
	close(ch)
}

// BroadcastConnectionUpdate sends a connection update to all subscribers
func (sm *SubscriptionManager) BroadcastConnectionUpdate(connection *model.Connection) {
	sm.connectionMutex.RLock()
	defer sm.connectionMutex.RUnlock()

	// Notify both users involved in the connection
	userIDs := []string{connection.UserID, connection.TargetUserID}

	for _, userID := range userIDs {
		if subscribers, ok := sm.connectionSubscribers[userID]; ok {
			for ch := range subscribers {
				select {
				case ch <- connection:
					// Connection update sent successfully
				default:
					// Channel is full, skip this subscriber
				}
			}
		}
	}
}

// Presence Subscription Methods

// SubscribeToPresence subscribes to presence updates for a user
func (sm *SubscriptionManager) SubscribeToPresence(userID string) (<-chan *model.PresenceUpdate, func()) {
	sm.presenceMutex.Lock()
	defer sm.presenceMutex.Unlock()

	ch := make(chan *model.PresenceUpdate, 10) // Buffered channel

	if sm.presenceSubscribers[userID] == nil {
		sm.presenceSubscribers[userID] = make(map[chan *model.PresenceUpdate]bool)
	}
	sm.presenceSubscribers[userID][ch] = true

	// Return channel and cleanup function
	cleanup := func() {
		sm.UnsubscribeFromPresence(userID, ch)
	}

	return ch, cleanup
}

func (sm *SubscriptionManager) UnsubscribeFromPresence(userID string, ch chan *model.PresenceUpdate) {
	sm.presenceMutex.Lock()
	defer sm.presenceMutex.Unlock()

	if subscribers, ok := sm.presenceSubscribers[userID]; ok {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(sm.presenceSubscribers, userID)
		}
	}
	close(ch)
}

func (sm *SubscriptionManager) BroadcastPresenceUpdate(userID string, isOnline bool, lastOnline *time.Time) {
	sm.presenceMutex.RLock()
	defer sm.presenceMutex.RUnlock()

	presenceUpdate := &model.PresenceUpdate{
		UserID:   userID,
		IsOnline: isOnline,
	}

	if lastOnline != nil {
		lastOnlineStr := lastOnline.Format(time.RFC3339)
		presenceUpdate.LastOnline = &lastOnlineStr
	}

	// Broadcast to all subscribers of this user's presence
	if subscribers, ok := sm.presenceSubscribers[userID]; ok {
		for ch := range subscribers {
			select {
			case ch <- presenceUpdate:
				// Presence update sent successfully
			default:
				// Channel is full, skip this subscriber
			}
		}
	}
}

// Typing Subscription Methods

// SubscribeToTyping subscribes to typing status updates for a chat
func (sm *SubscriptionManager) SubscribeToTyping(chatID string) (<-chan *model.TypingStatus, func()) {
	sm.typingMutex.Lock()
	defer sm.typingMutex.Unlock()

	ch := make(chan *model.TypingStatus, 10) // Buffered channel

	if sm.typingSubscribers[chatID] == nil {
		sm.typingSubscribers[chatID] = make(map[chan *model.TypingStatus]bool)
	}
	sm.typingSubscribers[chatID][ch] = true

	// Return channel and cleanup function
	cleanup := func() {
		sm.UnsubscribeFromTyping(chatID, ch)
	}

	return ch, cleanup
}

// UnsubscribeFromTyping removes a subscription for typing status updates
func (sm *SubscriptionManager) UnsubscribeFromTyping(chatID string, ch chan *model.TypingStatus) {
	sm.typingMutex.Lock()
	defer sm.typingMutex.Unlock()

	if subscribers, ok := sm.typingSubscribers[chatID]; ok {
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(sm.typingSubscribers, chatID)
		}
	}
	close(ch)
}

func (sm *SubscriptionManager) BroadcastTypingStatus(chatID string, userID string, isTyping bool) {
	sm.typingMutex.RLock()
	defer sm.typingMutex.RUnlock()

	typingStatus := &model.TypingStatus{
		UserID:   userID,
		IsTyping: isTyping,
	}

	if subscribers, ok := sm.typingSubscribers[chatID]; ok {
		for ch := range subscribers {
			select {
			case ch <- typingStatus:
				// Typing status sent successfully
			default:
				// Channel is full, skip this subscriber
			}
		}
	}
}

// Cleanup method to close channels and prevent goroutine leaks
func (sm *SubscriptionManager) cleanup() {
	sm.messageMutex.Lock()
	defer sm.messageMutex.Unlock()

	// Close all message subscription channels
	for chatID, subscribers := range sm.messageSubscribers {
		for ch := range subscribers {
			close(ch)
		}
		delete(sm.messageSubscribers, chatID)
	}

	// Similar cleanup for other subscription types would go here
	// For brevity, just showing the pattern for messages
}
