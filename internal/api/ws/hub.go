package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/chat"
	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Hub struct {
	mu          sync.RWMutex
	clients     map[*Client]bool
	chatService *chat.Service
}

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	room       string
	mu         sync.Mutex
	done       bool
	chatSeq      int
	chatCancel context.CancelFunc
}

type Message struct {
	Type           string        `json:"type"`
	Content        string        `json:"content,omitempty"`
	Sources        []chat.Source `json:"sources,omitempty"`
	Done           bool          `json:"done,omitempty"`
	Error          string        `json:"error,omitempty"`
	ConversationID string        `json:"conversation_id,omitempty"`
	MessageID      string        `json:"message_id,omitempty"`
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*Client]bool)}
}

func (h *Hub) SetChatService(svc *chat.Service) {
	h.chatService = svc
}

func (h *Hub) BroadcastEvents(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		if c.room == "events" {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.trySend(data)
	}
}

func (h *Hub) Run() {
	for {
		time.Sleep(time.Hour)
	}
}

func (h *Hub) ServeChat(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "chat")
}

func (h *Hub) ServeEvents(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "events")
}

func (h *Hub) serve(w http.ResponseWriter, r *http.Request, room string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}

	client := &Client{hub: h, conn: conn, send: make(chan []byte, 256), room: room}
	h.register(client)

	go client.writePump()
	go client.readPump()
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
}

func (c *Client) disconnect() {
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return
	}
	c.done = true
	if c.chatCancel != nil {
		c.chatCancel()
		c.chatCancel = nil
	}
	close(c.send)
	c.mu.Unlock()

	c.hub.mu.Lock()
	delete(c.hub.clients, c)
	c.hub.mu.Unlock()
}

func (c *Client) readPump() {
	defer func() {
		c.disconnect()
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("websocket read: %v", err)
			}
			break
		}

		if c.room == "chat" {
			c.handleChat(data)
		}
	}
}

func (c *Client) handleChat(data []byte) {
	var req struct {
		Type           string `json:"type"`
		Content        string `json:"content"`
		ConversationID string `json:"conversation_id"`
	}
	if err := json.Unmarshal(data, &req); err != nil || req.Content == "" {
		c.sendJSON(Message{Type: "error", Error: "invalid message"})
		return
	}

	svc := c.hub.chatService
	if svc == nil {
		c.sendJSON(Message{Type: "error", Error: "chat service unavailable"})
		return
	}

	var convID uuid.UUID
	if req.ConversationID != "" {
		parsed, err := uuid.Parse(req.ConversationID)
		if err != nil {
			c.sendJSON(Message{Type: "error", Error: "invalid conversation_id"})
			return
		}
		convID = parsed
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)

		c.mu.Lock()
		if c.done {
			c.mu.Unlock()
			cancel()
			return
		}
		if c.chatCancel != nil {
			c.chatCancel()
		}
		c.chatSeq++
		seq := c.chatSeq
		c.chatCancel = cancel
		c.mu.Unlock()

		defer func() {
			c.mu.Lock()
			if c.chatSeq == seq {
				c.chatCancel = nil
			}
			c.mu.Unlock()
			cancel()
		}()

		err := svc.Chat(ctx, conversation.ChatRequest{
			Content:        req.Content,
			ConversationID: convID,
		}, func(out chat.Outbound) error {
			if !c.sendJSON(Message{
				Type:           out.Type,
				Content:        out.Content,
				Sources:        out.Sources,
				Done:           out.Done,
				Error:          out.Error,
				ConversationID: out.ConversationID,
				MessageID:      out.MessageID,
			}) {
				return context.Canceled
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			c.sendJSON(Message{Type: "error", Error: err.Error()})
		}
	}()
}

func (c *Client) sendJSON(msg Message) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	return c.trySend(data)
}

func (c *Client) trySend(data []byte) (ok bool) {
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return false
	}
	c.mu.Unlock()

	defer func() {
		if recover() != nil {
			ok = false
		}
	}()

	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
