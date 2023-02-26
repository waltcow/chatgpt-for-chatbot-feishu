package chatgptclient

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-zoox/lru"
)

// Client is the ChatGPT Client.
type Client interface {
	Ask(question []byte, cfg ...*AskConfig) (*ChatGPTBrowserResponse, error)
	GetOrCreateConversation(id string, cfg *ConversationConfig) (Conversation, error)
	ResetConversations() error
	ResetConversation(id string) error
}

type client struct {
	core               *http.Client
	cfg                *Config
	conversationsCache *lru.LRU
}

// Config is the configuration for the ChatGPT Client.
type Config struct {
	APIKey               string `json:"api_key"`
	APIServer            string `json:"api_server"`
	ProxyAPIServer       string `json:"proxy_api_server"`
	MaxConversations     int    `json:"max_conversations"`
	ConversationMaxAge   int    `json:"conversation_max_age"`
	ConversationContext  string `json:"conversation_context"`
	ConversationLanguage string `json:"conversation_language"`
}

// AskConfig ...
type AskConfig struct {
	Question        string `json:"question"`
	ConversationId  string `json:"conversation_id"`
	ParentMessageId string `json:"parent_message_id"`

	ClientId              string `json:"client_id"`
	ConversationSignature string `json:"conversation_signature"`
	InvocationId          int    `json:"invocation_id"`
}

// New creates a new ChatGPT Client.
func New(cfg *Config) (Client, error) {
	if cfg.MaxConversations == 0 {
		cfg.MaxConversations = DefaultMaxConversations
	}

	return &client{
		core:               createHTTPClient(),
		cfg:                cfg,
		conversationsCache: lru.New(cfg.MaxConversations),
	}, nil
}

func (c *client) Ask(question []byte, cfg ...*AskConfig) (r *ChatGPTBrowserResponse, err error) {
	questionX := string(question)

	if len(cfg) > 0 {
		if cfg[0].Question != "" {
			questionX = cfg[0].Question
		}
	}

	requestBody := map[string]interface{}{
		"message": questionX,
	}

	if len(cfg) > 0 {
		if cfg[0].ConversationId != "" {
			requestBody["conversationId"] = cfg[0].ConversationId
		}
		if cfg[0].ParentMessageId != "" {
			requestBody["parentMessageId"] = cfg[0].ParentMessageId
		}
	}

	marshalStr, err := json.Marshal(requestBody)

	if err != nil {
		return nil, err
	}

	resp, err := c.core.Post(c.cfg.ProxyAPIServer, "application/json", strings.NewReader(string(marshalStr)))

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var result ChatGPTBrowserResponse

	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *client) GetOrCreateConversation(id string, cfg *ConversationConfig) (conversation Conversation, err error) {
	if cfg.ID == "" {
		cfg.ID = id
	}

	if cfg.MaxAge == 0 {
		cfg.MaxAge = DefaultConversationMaxAge
	}

	if cache, ok := c.conversationsCache.Get(cfg.ID); ok {
		if c, ok := cache.(Conversation); ok {
			conversation = c
			return conversation, nil
		}
	}

	conversation, err = NewConversation(c, cfg)
	if err != nil {
		return nil, err
	}

	c.conversationsCache.Set(id, conversation, cfg.MaxAge)

	return conversation, nil
}

func (c *client) ResetConversations() error {
	c.conversationsCache.Clear()

	return nil
}

func (c *client) ResetConversation(id string) error {
	c.conversationsCache.Delete(id)

	return nil
}

func createHTTPClient() *http.Client {
	defaultTransport := http.DefaultTransport.(*http.Transport)
	customTransport := defaultTransport.Clone()
	return &http.Client{
		Transport: customTransport,
	}
}
