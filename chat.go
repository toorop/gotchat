package gotchat

import (
	"container/list"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"path/filepath"

	"golang.org/x/net/websocket"

	"github.com/boltdb/bolt"
)

var boltDB *bolt.DB
var logger *log.Logger
var debug bool

// Message is the structure used for the dialog between websocket client and server
type Message struct {
	Action string `json:"action"`
	Txt    string `json:"txt"`
}

// Config represents global configuration for chat
type Config struct {
	// log
	Logger *log.Logger

	// debug
	Debug bool

	// data path for storing data (archives, DB files, template....)
	// default = appPath/data
	DataPath string
}

// Chat represents a chat
type Chat struct {
	rooms    map[string]*Room
	dataPath string
}

// NewChat returm a new Chat
func NewChat(c Config) (chat *Chat, err error) {
	chat = new(Chat)
	// check config

	// Logger
	if c.Logger != nil {
		logger = c.Logger
	} else {
		logger = log.New(os.Stdout, "[gotchat]", log.LstdFlags)
	}

	// debug
	debug = c.Debug

	//DataPath
	if c.DataPath == "" {
		var basePath string
		basePath, err = filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			return
		}
		c.DataPath = path.Join(basePath, "data")
	}
	// Exists
	if _, err = os.Stat(c.DataPath); err != nil {
		// try to create it
		if err = os.MkdirAll(c.DataPath, 0700); err != nil {
			return
		}
	}
	chat.dataPath = c.DataPath

	// init BoltDB
	boltDB, err = bolt.Open(path.Join(chat.dataPath, "bolt.db"), 0600, nil)
	if err != nil {
		return
	}
	// rooms
	chat.rooms = make(map[string]*Room)
	return chat, nil
}

// WebsocketHandler handle webssocket conn
func (c *Chat) WebsocketHandler(ws *websocket.Conn) {
	// 1 WS conn handle 1 user in 1 room
	// nic of the curent subscriber
	var nic string
	// chatroom handle by this con
	var room *Room
	// rawMsg message received from client
	var rawMsg string
	// msg recieved from client
	var msg msgCommon

	// defer
	defer func() {
		// remove subscriber
		if nic != "" && room != nil {
			room.Unsub(nic)
			logInfo(nic + " leave chatroom")
		}
		ws.Close()
	}()

	// Handle WS messages
	for {
		if err := websocket.Message.Receive(ws, &rawMsg); err != nil {
			logTrace(err.Error())
			if err.Error() != "EOF" {
				logErr(err.Error())
			}
			return
		}

		// heartbeat
		if rawMsg == "p" {
			if room != nil {
				room.BeatReceivedFrom(nic)
				continue
			}
		}

		logTrace("-> " + rawMsg)

		// unmarshall message
		if err := json.Unmarshal([]byte(rawMsg), &msg); err != nil {
			logErr("unable to unmarshal message from client - " + err.Error())
			continue
		}

		//
		switch msg.Cmd {
		// Join chatroom
		case "join":
			var err error
			// msg.txt is a JSON object
			var data struct {
				Nic  string `json:"nic" `
				Room string `json:"room"`
			}
			if err := json.Unmarshal([]byte(msg.Data), &data); err != nil {
				logErr("unable to unmarshall " + msg.Data + " as data struct")
				wsSend(ws, "error", "internal server error")
				continue
			}
			// room exists ?
			// TODO  autocreate room if it doesn't exists
			room, err = c.GetRoom(data.Room)
			if err != nil {
				if err == ErrRoomNotFound {
					logErr(nic + " wants to join an inexisting room: " + data.Room)
					wsSend(ws, "error", data.Room+" doesn't exists")
				} else {
					logErr(nic + " can not join room " + data.Room)
					wsSend(ws, "error", "internal server error")
				}
				continue
			}

			nic = data.Nic
			if err := room.Sub(nic, ws); err != nil {
				logErr(err.Error())
				wsSend(ws, "error", "Unable to join room "+room.name+" "+err.Error())
				continue
			} else {
				wsSend(ws, "joinOK", "")
				logInfo(nic + " has join room " + room.name)
			}

			// Send archive
			room.PushArchivesTo(nic)
			room.PushMessage("chatbot", nic+` has joined the room`)
			continue

			// Publish in chatroom
		case "newmsg":
			// Si pas ne nic pas de publish
			if nic == "" || room == nil {
				continue
			}
			if err := room.NewIncomingMessageFrom(nic, msg.Data); err != nil {
				logErr(err.Error())
			}
			//room.PushMessage(nic + " " + msg.Txt)
			continue

		default:
			logInfo("websocket chatroom - unimplemented action send to websocket " + msg.Cmd)
			continue
		}
	}
}

// AddRoom add a new room
func (c *Chat) AddRoom(name string, conf RoomConfig) (err error) {
	// room exists ?
	if c.RoomExists(name) {
		return errors.New("room " + name + " exists")
	}

	// Archive
	if conf.Archived {
		// check if bucket exist
		err = boltDB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(name + "_arch"))
			return err
		})
		if err != nil {
			return err
		}
	}

	room := &Room{
		name:        name,
		subscribers: list.New(),
		archives:    list.New(),
		archiveLen:  conf.ArchivePushMessagesCount,
		heartrate:   conf.HeartRate,
	}

	if room.heartrate != 0 {
		room.Watch()
	}
	c.rooms[name] = room
	return nil
}

// RoomExists checks if room exists
func (c *Chat) RoomExists(name string) bool {
	_, exists := c.rooms[name]
	return exists
}

// GetRoom return a pointer to Room named name
func (c *Chat) GetRoom(name string) (room *Room, err error) {
	room, exists := c.rooms[name]
	if !exists {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// Log helpers
func logTrace(msg string) {
	if !debug {
		return
	}
	logger.Println("TRACE - " + msg)
}
func logInfo(msg string) {
	logger.Println("INFO - " + msg)
}
func logErr(msg string) {
	logger.Println("ERROR - " + msg)
}

// Websocket helpers

// send a formated reply
func wsSend(ws *websocket.Conn, action, txt string) {
	response, err := json.Marshal(msgCommon{
		Cmd:  action,
		Data: txt,
	})
	if err == nil {
		ws.Write(response)
	}
}
