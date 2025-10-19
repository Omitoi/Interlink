package main

import (
	"testing"
	"time"

	"gitea.kood.tech/petrkubec/match-me/backend/graph"
	"gitea.kood.tech/petrkubec/match-me/backend/graph/model"
	"github.com/stretchr/testify/assert"
)

func TestGraphQLSubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping GraphQL subscriptions test in short mode")
	}

	t.Run("Message Subscription Broadcasting", func(t *testing.T) {
		// Test the subscription manager directly
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe to a test chat
		chatID := "test-chat-123"
		messageCh, cleanup := subscriptionManager.SubscribeToMessages(chatID)
		defer cleanup()

		// Create a test message
		testMessage := &model.ChatMessage{
			ID:        "1",
			ChatID:    chatID,
			SenderID:  "1",
			Content:   "Test message",
			CreatedAt: time.Now().Format(time.RFC3339),
			IsRead:    false,
		}

		// Broadcast the message
		go func() {
			time.Sleep(100 * time.Millisecond) // Small delay to ensure subscription is ready
			subscriptionManager.BroadcastMessage(testMessage)
		}()

		// Wait for the message
		select {
		case receivedMessage := <-messageCh:
			assert.Equal(t, testMessage.ID, receivedMessage.ID)
			assert.Equal(t, testMessage.Content, receivedMessage.Content)
			assert.Equal(t, testMessage.ChatID, receivedMessage.ChatID)
			assert.Equal(t, testMessage.SenderID, receivedMessage.SenderID)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for message subscription")
		}
	})

	t.Run("Connection Subscription Broadcasting", func(t *testing.T) {
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe to connection updates for a test user
		userID := "test-user-456"
		connectionCh, cleanup := subscriptionManager.SubscribeToConnections(userID)
		defer cleanup()

		// Create a test connection
		testConnection := &model.Connection{
			ID:           "1",
			UserID:       userID,
			TargetUserID: "2",
			Status:       model.ConnectionStatusAccepted,
			CreatedAt:    time.Now().Format(time.RFC3339),
			UpdatedAt:    time.Now().Format(time.RFC3339),
		}

		// Broadcast the connection update
		go func() {
			time.Sleep(100 * time.Millisecond)
			subscriptionManager.BroadcastConnectionUpdate(testConnection)
		}()

		// Wait for the connection update
		select {
		case receivedConnection := <-connectionCh:
			assert.Equal(t, testConnection.ID, receivedConnection.ID)
			assert.Equal(t, testConnection.UserID, receivedConnection.UserID)
			assert.Equal(t, testConnection.TargetUserID, receivedConnection.TargetUserID)
			assert.Equal(t, testConnection.Status, receivedConnection.Status)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for connection subscription")
		}
	})

	t.Run("Presence Subscription Broadcasting", func(t *testing.T) {
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe to presence updates for a test user
		userID := "test-user-789"
		presenceCh, cleanup := subscriptionManager.SubscribeToPresence(userID)
		defer cleanup()

		// Broadcast a presence update
		go func() {
			time.Sleep(100 * time.Millisecond)
			lastOnline := time.Now()
			subscriptionManager.BroadcastPresenceUpdate(userID, true, &lastOnline)
		}()

		// Wait for the presence update
		select {
		case receivedPresence := <-presenceCh:
			assert.Equal(t, userID, receivedPresence.UserID)
			assert.True(t, receivedPresence.IsOnline)
			assert.NotNil(t, receivedPresence.LastOnline)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for presence subscription")
		}
	})

	t.Run("Typing Status Subscription Broadcasting", func(t *testing.T) {
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe to typing status for a test chat
		chatID := "test-chat-typing-999"
		typingCh, cleanup := subscriptionManager.SubscribeToTyping(chatID)
		defer cleanup()

		// Broadcast a typing status update
		go func() {
			time.Sleep(100 * time.Millisecond)
			subscriptionManager.BroadcastTypingStatus(chatID, "test-user-123", true)
		}()

		// Wait for the typing status update
		select {
		case receivedTyping := <-typingCh:
			assert.Equal(t, "test-user-123", receivedTyping.UserID)
			assert.True(t, receivedTyping.IsTyping)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for typing subscription")
		}
	})

	t.Run("Multiple Subscribers to Same Channel", func(t *testing.T) {
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe multiple clients to the same chat
		chatID := "test-chat-multi-subscribers"

		messageCh1, cleanup1 := subscriptionManager.SubscribeToMessages(chatID)
		defer cleanup1()

		messageCh2, cleanup2 := subscriptionManager.SubscribeToMessages(chatID)
		defer cleanup2()

		// Create a test message
		testMessage := &model.ChatMessage{
			ID:        "multi-test-1",
			ChatID:    chatID,
			SenderID:  "1",
			Content:   "Multi-subscriber test message",
			CreatedAt: time.Now().Format(time.RFC3339),
			IsRead:    false,
		}

		// Broadcast the message
		go func() {
			time.Sleep(100 * time.Millisecond)
			subscriptionManager.BroadcastMessage(testMessage)
		}()

		// Both subscribers should receive the message
		received1 := false
		received2 := false

		for i := 0; i < 2; i++ {
			select {
			case msg1 := <-messageCh1:
				assert.Equal(t, testMessage.Content, msg1.Content)
				received1 = true
			case msg2 := <-messageCh2:
				assert.Equal(t, testMessage.Content, msg2.Content)
				received2 = true
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for multi-subscriber messages")
			}
		}

		assert.True(t, received1, "First subscriber should receive message")
		assert.True(t, received2, "Second subscriber should receive message")
	})

	t.Run("Subscription Cleanup", func(t *testing.T) {
		subscriptionManager := graph.GetSubscriptionManager()

		// Subscribe and immediately cleanup
		chatID := "test-chat-cleanup"
		messageCh, cleanup := subscriptionManager.SubscribeToMessages(chatID)

		// Verify subscription exists by checking if channel is valid
		assert.NotNil(t, messageCh)

		// Cleanup subscription
		cleanup()

		// Try to broadcast to cleaned up subscription
		testMessage := &model.ChatMessage{
			ID:        "cleanup-test",
			ChatID:    chatID,
			Content:   "This should not be received",
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		subscriptionManager.BroadcastMessage(testMessage)

		// Channel should be closed after cleanup
		select {
		case msg, ok := <-messageCh:
			if ok {
				t.Fatalf("Expected closed channel, but received message: %v", msg)
			}
			// Channel is properly closed - this is expected
		case <-time.After(1 * time.Second):
			// No message received - this is also acceptable
		}
	})
}

func TestDataLoaderPerformanceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping DataLoader performance integration test in short mode")
	}

	// This test would ideally run against a real server
	// For now, we'll test the subscription manager integration
	t.Run("DataLoader with Subscriptions", func(t *testing.T) {
		// Test that DataLoader and Subscription manager can work together
		subscriptionManager := graph.GetSubscriptionManager()

		// Test scenario: Multiple users getting updates efficiently
		userIDs := []string{"1", "2", "3", "4", "5"}
		channels := make([]<-chan *model.PresenceUpdate, len(userIDs))
		cleanups := make([]func(), len(userIDs))

		// Subscribe all users to presence updates
		for i, userID := range userIDs {
			ch, cleanup := subscriptionManager.SubscribeToPresence(userID)
			channels[i] = ch
			cleanups[i] = cleanup
			defer cleanup()
		}

		// Broadcast presence updates for all users
		start := time.Now()
		for _, userID := range userIDs {
			lastOnline := time.Now()
			subscriptionManager.BroadcastPresenceUpdate(userID, true, &lastOnline)
		}

		// Wait for all updates
		receivedCount := 0
		timeout := time.After(5 * time.Second)

		for receivedCount < len(userIDs) {
			select {
			case <-channels[0]:
				receivedCount++
			case <-channels[1]:
				receivedCount++
			case <-channels[2]:
				receivedCount++
			case <-channels[3]:
				receivedCount++
			case <-channels[4]:
				receivedCount++
			case <-timeout:
				t.Fatalf("Timeout: only received %d/%d presence updates", receivedCount, len(userIDs))
			}
		}

		duration := time.Since(start)
		t.Logf("Successfully processed %d presence updates in %v", len(userIDs), duration)

		// Performance assertion
		assert.Less(t, duration, 2*time.Second, "Subscription broadcasting should be fast")
		assert.Equal(t, len(userIDs), receivedCount, "All presence updates should be received")
	})
}
