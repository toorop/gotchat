package gotchat

// msgCommonResponse represents the base structure for each message sending to client
type msgCommon struct {
	Cmd  string `json:"cmd"`
	Data string `json:"data,omitempty"`
}

// messsage send from a user to the room
type msgFromUserToChatroom struct {
	msgCommon
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	User      string `json:"user"`
}
