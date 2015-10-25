package gotchat

import (
	"container/list"
	"encoding/json"
	"errors"
	"html"

	"golang.org/x/net/websocket"

	"fmt"
	"log"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/mvdan/xurls"
	"github.com/satori/go.uuid"
)

// RoomConfig is used to configure a new room
type RoomConfig struct {
	// do we have to archive this room
	// Default: false
	Archived bool
	// number of archived Messages server will push to new subscriber
	// Default 0
	ArchivePushMessagesCount uint16
	// if server do not recieve beat from sunscriber after this 2*delay (in sec)
	// subscriber will be unsub
	// Default 0
	HeartRate uint16
}

// Room represents a romm
type Room struct {
	name        string
	subscribers *list.List
	archives    *list.List
	archiveLen  uint16
	heartrate   uint16
}

// Sub add nic to chatroom
func (r *Room) Sub(nic string, ws *websocket.Conn) error {
	// check nick
	lenNic := len(nic)
	// must be at least 3 char
	if lenNic < 3 {
		return errors.New("Nickname must be at least 3 chars long")
	}
	// nick exists ?
	if r.HaveSubscriber(nic) || strings.ToLower(nic) == "chatbot" {
		return errors.New(nic + " already in chatroom. Choose another nic")
	}
	r.subscribers.PushFront(&subscriber{
		Nic:    NormalizeNic(nic),
		RawNic: nic,
		WS:     ws,
	})
	//r.PushMessage("chatbot", nic+` has joined the room`)
	return nil
}

// Unsub unsubscribe nic from chatroom
func (r *Room) Unsub(nic string) {
	nic = NormalizeNic(nic)
	for e := r.subscribers.Front(); e != nil; e = e.Next() {
		if e.Value.(*subscriber).Nic == nic {
			e.Value.(*subscriber).WS.Close()
			r.subscribers.Remove(e)
			r.PushMessage("chatbot", e.Value.(*subscriber).RawNic+` has left the room`)
		}
	}
}

// HaveSubscriber check if nic is in chatroom
func (r *Room) HaveSubscriber(nic string) bool {
	nic = NormalizeNic(nic)
	for e := r.subscribers.Front(); e != nil; e = e.Next() {
		if e.Value.(*subscriber).Nic == nic {
			return true
		}
	}
	return false
}

// GetSubscriber return subscriber... or not
func (r *Room) GetSubscriber(nic string) (*subscriber, error) {
	nic = NormalizeNic(nic)
	for e := r.subscribers.Front(); e != nil; e = e.Next() {
		if e.Value.(*subscriber).Nic == nic {
			return e.Value.(*subscriber), nil
		}
	}
	return nil, errors.New("not found")
}

// NewIncomingMessageFrom handle new message from nic
func (r *Room) NewIncomingMessageFrom(nic, msg string) error {
	msg = strings.TrimSpace(msg)
	// command ?
	if msg[0] == 47 {
		log.Println("command recieved")
		parts := strings.Fields(msg)
		// todo to lower
		switch parts[0] {
		case "/me":
			r.PushMessage("chatbot", nic)
		case "/users":
			log.Println("on est dans users")
			r.PushMessageTo(`Users list:`, nic)
			for e := r.subscribers.Front(); e != nil; e = e.Next() {
				if e.Value.(*subscriber).Nic != nic {
					r.PushMessageTo(`<li class="text-muted"><small>`+e.Value.(*subscriber).Nic+` </small></li>`, nic)
				}
			}
		}
	} else {
		// send message to chatroom
		return r.PushMessage(nic, msg)
	}
	return nil
}

// PushMessage push msg to all subscribers
func (r *Room) PushMessage(from, msg string) error {
	msg = r.formatMessage(msg)
	response := msgFromUserToChatroom{
		msgCommon{
			Cmd:  "newchatmsg",
			Data: msg,
		},
		uuid.NewV4().String(),
		time.Now().Unix(),
		from,
	}
	toSend, err := json.Marshal(response)
	if err != nil {
		return err
	}
	r.archive(toSend)
	for e := r.subscribers.Front(); e != nil; e = e.Next() {
		go func(sub *subscriber) {
			if _, err := sub.WS.Write(toSend); err != nil {
				r.Unsub(sub.Nic)
			}
		}(e.Value.(*subscriber))
	}
	return nil
}

// PushMessageTo send a message to nic
func (r *Room) PushMessageTo(msg, nic string) {
	logInfo("On a un message a pusher à " + nic + " " + msg)
	msg = r.formatMessage(msg)
	sub, err := r.GetSubscriber(nic)
	if err != nil {
		r.Unsub(nic)
	}
	message := Message{
		Action: "newchatmsg",
		Txt:    msg,
	}
	toSend, err := json.Marshal(message)
	if err != nil {
		return
	}
	sub.WS.Write(toSend)
}

// make link clickable and escape msg
func (r *Room) formatMessage(msg string) string {
	links := xurls.Relaxed.FindAllString(msg, -1)
	msg = html.EscapeString(msg)
	for _, link := range links {
		href := link
		if !strings.HasPrefix(href, "http") {
			href = "http://" + href
		}
		msg = strings.Replace(msg, link, `<a href="`+href+`" target="_blank">`+link+`</a>`, 1)
	}
	return msg
}

// archive add message to archives
func (r *Room) archive(message []byte) {
	err := boltDB.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(r.name + "_arch"))
		err := b.Put([]byte(fmt.Sprintf("%d", time.Now().UnixNano())), message)
		return err
	})
	if err != nil {
		logErr("unable to save archive " + err.Error())
	}
}

// PushArchivesTo send archive to subscriber
func (r *Room) PushArchivesTo(nic string) {
	sub, err := r.GetSubscriber(nic)
	if err != nil {
		return
	}
	archives := list.New()

	boltDB.View(func(tx *bolt.Tx) error {
		// Assume our events bucket has RFC3339 encoded time keys.
		c := tx.Bucket([]byte(r.name + "_arch")).Cursor()
		i := r.archiveLen
		for k, v := c.Last(); k != nil && i > 0; k, v = c.Prev() {
			archives.PushFront(v)
			i--
		}
		return nil
	})

	for e := archives.Front(); e != nil; e = e.Next() {
		sub.WS.Write(e.Value.([]byte))
	}
}

// Watch checks if subscribers are always here
// normaly useless if you do not use proxy
func (r *Room) Watch() {
	// remove ghosts (disconnected subscribers with stalling websocket connexion)
	ping := []byte{112}
	beginingOfTheWorld := time.Time{}
	go func() {
		for {
			for e := r.subscribers.Front(); e != nil; e = e.Next() {
				// send ping and check heartbeat
				go func(sub *subscriber) {
					//logTrace("ping sent to " + sub.Nic)
					if _, err := sub.WS.Write(ping); err != nil {
						r.Unsub(sub.Nic)
					}
					// heartbeat
					// if no heartbeat for r.heartrate * 2 => subscriber is DEAD ! RIP
					//log.Println(sub.Lastbeat)
					//log.Println(time.Since(sub.Lastbeat).Seconds(), "-", float64(2*r.heartrate))
					if sub.Lastbeat != beginingOfTheWorld && time.Since(sub.Lastbeat).Seconds() > float64(2*r.heartrate) {
						logInfo(sub.Nic + " is dead. RIP")
						r.Unsub(sub.Nic)
					}
				}(e.Value.(*subscriber))
			}
			time.Sleep(time.Duration(r.heartrate) * time.Second)
		}
	}()
}

// BeatReceivedFrom handle beat recived from subscriber
func (r *Room) BeatReceivedFrom(nic string) {
	//logTrace("beat received from " + nic)
	sub, err := r.GetSubscriber(nic)
	if err != nil {
		log.Println("unable to find", nic, "nic", err)
		if sub != nil {
			r.Unsub(sub.Nic)
		}
		return
	}
	sub.Lastbeat = time.Now()
}
