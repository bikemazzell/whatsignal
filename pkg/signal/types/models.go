package types

type SendMessageRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Number      string   `json:"number"`
		Message     string   `json:"message"`
		Attachments []string `json:"attachments,omitempty"`
	} `json:"params"`
	ID int `json:"id"`
}

type SendMessageResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  struct {
		Timestamp int64  `json:"timestamp"`
		MessageID string `json:"messageId"`
	} `json:"result"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}

type ReceiveMessageRequest struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Timeout int `json:"timeout"`
	} `json:"params"`
	ID int `json:"id"`
}

type SignalMessage struct {
	Timestamp     int64    `json:"timestamp"`
	Sender        string   `json:"sender"`
	MessageID     string   `json:"messageId"`
	Message       string   `json:"message"`
	Attachments   []string `json:"attachments"`
	QuotedMessage *struct {
		ID        string `json:"id"`
		Author    string `json:"author"`
		Text      string `json:"text"`
		Timestamp int64  `json:"timestamp"`
	} `json:"quotedMessage,omitempty"`
}

type ReceiveMessageResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Result  []SignalMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID int `json:"id"`
}
