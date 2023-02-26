package chatgptclient

import (
	"fmt"
	"time"

	"github.com/go-zoox/core-utils/safe"
	"github.com/go-zoox/logger"
	"github.com/go-zoox/uuid"
)

// Conversation is the conversation interface.
type Conversation interface {
	Ask(question []byte, cfg ...*ConversationAskConfig) (answer []byte, err error)
	IsQuestionAsked(id string) (err error)
	ID() string
	Messages() *safe.List
}

type conversation struct {
	client      *client
	id          string
	messages    *safe.List
	messagesMap *safe.Map
	cfg         *ConversationConfig

	LastMessageId  string
	ConversationId string
	// bing parts
	InvocationId          int
	ClientId              string
	ConversationSignature string
}

// ConversationConfig is the configuration for creating a new Conversation.
type ConversationConfig struct {
	ID               string
	ConversationID   string
	Context          string
	Language         string
	MaxMessages      int
	MaxAge           time.Duration
	MaxRequestTokens int
	Model            string `json:"model"`
}

// ConversationAskConfig is the configuration for ask question.
type ConversationAskConfig struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	User           string    `json:"user"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewConversation creates a new Conversation.
func NewConversation(client *client, cfg *ConversationConfig) (Conversation, error) {
	if cfg.ID == "" {
		cfg.ID = uuid.V4()
	}

	if cfg.MaxMessages == 0 {
		cfg.MaxMessages = 100
	}

	return &conversation{
		client:      client,
		id:          cfg.ID,
		messages:    safe.NewList(cfg.MaxMessages),
		messagesMap: safe.NewMap(),
		cfg:         cfg,
	}, nil
}

func (c *conversation) IsQuestionAsked(id string) (err error) {
	if c.messagesMap.Has(id) {
		return fmt.Errorf("duplicate message(id: %s) to ask", id)
	}

	return nil
}

func (c *conversation) Ask(question []byte, cfg ...*ConversationAskConfig) (answer []byte, err error) {
	cfgX := &ConversationAskConfig{}
	if len(cfg) > 0 && cfg[0] != nil {
		cfgX = cfg[0]
	}
	if cfgX.ID == "" {
		logger.Warnf("question id is empty, generating a new one")
		cfgX.ID = uuid.V4()
	}

	if cfgX.CreatedAt.IsZero() {
		cfgX.CreatedAt = time.Now()
	}

	if c.messagesMap.Has(cfgX.ID) {
		return nil, fmt.Errorf("duplicate message(id: %s) to ask", cfgX.ID)
	}

	c.messagesMap.Set(cfgX.ID, true)

	c.messages.Push(&Message{
		ID:             cfgX.ID,
		Text:           string(question),
		IsChatGPT:      false,
		ConversationID: c.cfg.ConversationID,
		User:           cfgX.User,
		CreatedAt:      cfgX.CreatedAt,
	})

	askConf := &AskConfig{}

	if c.ConversationId != "" {
		askConf.ConversationId = c.ConversationId
		askConf.ParentMessageId = c.LastMessageId
		// bing parts
		askConf.ClientId = c.ClientId
		askConf.ConversationSignature = c.ConversationSignature
		askConf.InvocationId = c.InvocationId
	}

	result, err := c.client.Ask(question, askConf)

	if err != nil {
		return nil, fmt.Errorf("failed to ask: %v", err)
	}

	c.messages.Push(&Message{
		ID:             uuid.V4(),
		Text:           result.Response,
		IsChatGPT:      true,
		ConversationID: result.ConversationId,
	})

	c.LastMessageId = result.MessageId
	c.ConversationId = result.ConversationId

	if result.IsBingResponse() {
		c.ConversationSignature = result.ConversationSignature
		c.InvocationId = result.InvocationId
		c.ClientId = result.ClientId
	}

	return []byte(result.Response), nil
}

func (c *conversation) ID() string {
	return c.id
}

func (c *conversation) Messages() *safe.List {
	return c.messages
}
