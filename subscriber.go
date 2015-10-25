package gotchat

import (
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

type subscriber struct {
	Nic      string
	RawNic   string
	WS       *websocket.Conn
	Lastbeat time.Time
}

// NormalizeNic return a normalized version of nic
// cut to 10 char && tolower
func NormalizeNic(nic string) string {
	// must be < 10
	if len(nic) > 10 {
		nic = nic[0:10]
	}
	return strings.ToLower(nic)
}
