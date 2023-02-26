package chatgptclient

// ChatGPTBrowserResponse { response: 'Hi! How can I help you today?', conversationId: '...', messageId: '...' }
type ChatGPTBrowserResponse struct {
	Response       string `json:"response"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`

	// bing parts
	InvocationId          string `json:"invocationId"`
	ClientId              string `json:"clientId"`
	ConversationSignature string `json:"conversationSignature"`
}

func (response *ChatGPTBrowserResponse) IsBingResponse() bool {
	return response.InvocationId != "" && response.ClientId != "" && response.ConversationSignature != ""
}
