package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/toorop/gotchat"

	"golang.org/x/net/websocket"
)

var chat *gotchat.Chat

// handleIndex home handler
func handleIndex(w http.ResponseWriter, r *http.Request) {
	reader, _ := os.Open("index.html")
	io.Copy(w, reader)
}

// handleSound return notification  sound
func handleSound(w http.ResponseWriter, r *http.Request) {
	reader, _ := os.Open("chat.wav")
	io.Copy(w, reader)
}

// handleJS return javascript
func handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/javascript")
	reader, _ := os.Open("chat.js")
	io.Copy(w, reader)
}

// Init
func init() {
	var err error
	chatConfig := gotchat.Config{
		Debug: true,
	}
	chat, err = gotchat.NewChat(chatConfig)
	if err != nil {
		panic(err)
	}
	// add room test
	roomConfig := gotchat.RoomConfig{
		Archived:                 true,
		ArchivePushMessagesCount: 50,
		HeartRate:                1,
	}
	err = chat.AddRoom("test", roomConfig)
	if err != nil {
		panic(err)
	}
}

// main function
func main() {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/js", handleJS)
	http.HandleFunc("/sound", handleSound)
	http.Handle("/ws", websocket.Handler(chat.WebsocketHandler))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
