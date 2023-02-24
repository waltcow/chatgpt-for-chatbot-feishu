package chatgptclient

// ChatGPTBrowserResponse { response: 'Hi! How can I help you today?', conversationId: '...', messageId: '...' }
type ChatGPTBrowserResponse struct {
	Response       string `json:"response"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
}
