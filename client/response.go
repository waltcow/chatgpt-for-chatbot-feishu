package chatgptclient

// ChatGPTBrowserResponse { response: 'Hi! How can I help you today?', conversationId: '...', messageId: '...' }
type ChatGPTBrowserResponse struct {
	Response       string `json:"response"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`

	// bing parts
	InvocationId          int    `json:"invocationId"`
	ClientId              string `json:"clientId"`
	ConversationSignature string `json:"conversationSignature"`
}

func (response *ChatGPTBrowserResponse) IsBingResponse() bool {
	return response.InvocationId > 0 && response.ClientId != "" && response.ConversationSignature != ""
}
