// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nlopes/slack"
)

type EventHandler interface {
	Event(evt slack.SlackEvent) (handled bool)
}

type Room interface {
	EventHandler
	IsOpen() bool
	//Open()  // maybe?
	//Close() // maybe?
	Id() string
	Name() string
}

type FSConn struct {
	super *Super

	api *slack.Slack
	ws  *slack.SlackWS
	in  chan slack.SlackEvent

	sinks    []EventHandler
	users    *UserSet
	channels *RoomSet
	groups   *RoomSet
}

// shared by offline/offline public New functions
func newFSConn(token, infoPath string) (conn *FSConn, err error) {
	var info slack.Info
	conn = new(FSConn)

	if infoPath != "" {
		buf, err := ioutil.ReadFile(infoPath)
		if err != nil {
			return nil, fmt.Errorf("ReadFile(%s): %s", infoPath, err)
		}
		err = json.Unmarshal(buf, &info)
		if err != nil {
			return nil, fmt.Errorf("Unmarshal: %s", err)
		}
	} else {
		conn.api = slack.New(token)
		//conn.api.SetDebug(true)
		conn.ws, info, err = conn.api.StartRTM("", "https://slack.com")
		if err != nil {
			return nil, fmt.Errorf("StartRTM(): %s\n", err)
		}
	}

	conn.in = make(chan slack.SlackEvent)
	conn.sinks = make([]EventHandler, 0, 4)
	conn.super = NewSuper()

	users := make([]*User, 0, len(info.Users))
	for _, u := range info.Users {
		users = append(users, NewUser(u, conn))
	}
	conn.users, err = NewUserSet("users", conn, NewUserDir, users)
	if err != nil {
		return nil, fmt.Errorf("NewUserSet: %s", err)
	}

	chans := make([]Room, 0, len(info.Channels))
	for _, c := range info.Channels {
		chans = append(chans, NewChannel(c, conn))
	}
	conn.channels, err = NewRoomSet("channels", conn, NewChannelDir, chans)
	if err != nil {
		return nil, fmt.Errorf("NewRoomSet: %s", err)
	}

	groups := make([]Room, 0, len(info.Groups))
	for _, g := range info.Groups {
		groups = append(groups, NewGroup(g, conn))
	}
	conn.groups, err = NewRoomSet("groups", conn, NewGroupDir, groups)
	if err != nil {
		return nil, fmt.Errorf("NewRoomSet: %s", err)
	}

	// simplify dispatch code by keeping track of event handlers
	// in a slice.  We (FSConn) are an event sink too - add
	// ourselves to the list first, so that we can separate
	// routing logic from connection-level handling logic.
	conn.sinks = append(conn.sinks, conn, conn.channels, conn.groups, conn.users)

	// only spawn goroutines in online mode
	if infoPath == "" {
		go conn.ws.HandleIncomingEvents(conn.in)
		go conn.ws.Keepalive(10 * time.Second)
		go conn.consumeEvents()
	}

	return conn, nil
}

func NewFSConn(token string) (*FSConn, error) {
	return newFSConn(token, "")
}

func NewOfflineFSConn(infoPath string) (*FSConn, error) {
	return newFSConn("", infoPath)
}

func (conn *FSConn) Event(evt slack.SlackEvent) bool {
	switch evt.Data.(type) {
	case slack.HelloEvent, slack.LatencyReport:
		// TODO: keep track of potential disconnects.
		return true
	}
	return false
}

func (conn *FSConn) consumeEvents() {
	for {
		evt := <-conn.in
		go conn.routeEvent(evt)
	}
}

func (conn *FSConn) routeEvent(evt slack.SlackEvent) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("routeEvent panic: %s", err)
		}
	}()

	var ok bool
	for _, handler := range conn.sinks {
		if ok = handler.Event(evt); ok {
			break
		}
	}
	if !ok {
		fmt.Printf("unhandled evt: %#v\n", evt)
	}
}

func (fs *FSConn) Send(txtBytes []byte, id string) error {
	txt := strings.TrimSpace(string(txtBytes))

	out := fs.ws.NewOutgoingMessage(txt, id)
	err := fs.ws.SendMessage(out)
	if err != nil {
		log.Printf("SendMessage: %s", err)
	}
	// TODO(bp) add this message to the session buffer, after we
	// get an ok
	return err
}

type UserSet struct {
	sync.Mutex
	objs map[string]*User
	ds   *DirSet
	conn *FSConn
}

func NewUserSet(name string, conn *FSConn, create DirCreator, users []*User) (*UserSet, error) {
	var err error
	us := new(UserSet)
	us.conn = conn
	us.objs = make(map[string]*User)
	us.ds, err = NewDirSet(conn.super.root, name, create, conn)
	if err != nil {
		return nil, fmt.Errorf("NewDirSet('groups'): %s", err)
	}

	for _, user := range users {
		us.objs[user.Id] = user
		// hide deleted users, they're not useful.
		if user.Deleted {
			continue
		}
		err = us.ds.Add(user.Id, user.Name, user)
		if err != nil {
			return nil, fmt.Errorf("Add(%s): %s", user.Id, err)
		}
	}

	us.ds.Activate()
	return us, nil
}

// on change, lock UserSet, then lock User.  No need to lock DirNode,
// as it doesn't change (we're not adding/removing child attributes).
// Updates to Attributes are done through atomic ops.
func (us *UserSet) Event(evt slack.SlackEvent) bool {
	switch msg := evt.Data.(type) {
	case *slack.ManualPresenceChangeEvent:
		// ignore - when we change our own presence, we get
		// both a manual and non-manual event, so we handle
		// this the same way as a presence change for anyone
		// else.
		return true
	case *slack.PresenceChangeEvent:
		us.Lock()
		defer us.Unlock()

		user, ok := us.objs[msg.UserId]
		if !ok {
			log.Printf("XXX: presence change with no user object: %s", msg.UserId)
			return true
		}

		log.Printf("Presence Change: %s -> %s", user.Name, msg.Presence)

		ud := us.ds.LookupId(msg.UserId)
		if ud == nil {
			log.Printf("XXX: presence change for unknown user: %s", msg.UserId)
			return true
		}

		user.mu.Lock()
		defer user.mu.Unlock()

		user.Presence = msg.Presence
		for _, child := range ud.children {
			if up, ok := child.(Updater); ok && child.Name() == "presence" {
				up.Update()
			}
		}

		return true
	}
	return false
}

type RoomSet struct {
	sync.Mutex
	objs map[string]Room
	ds   *DirSet
	conn *FSConn
}

func NewRoomSet(name string, conn *FSConn, create DirCreator, rooms []Room) (*RoomSet, error) {
	var err error
	rs := new(RoomSet)
	rs.conn = conn
	rs.objs = make(map[string]Room)
	rs.ds, err = NewDirSet(conn.super.root, name, create, conn)
	if err != nil {
		return nil, fmt.Errorf("NewDirSet('groups'): %s", err)
	}

	for _, room := range rooms {
		rs.objs[room.Id()] = room
		// filesystem objects are created and destroyed based
		// on whether we are members of the given room (or in
		// the case of IMs and groups, whether the room is
		// 'open'.
		if !room.IsOpen() {
			continue
		}
		err = rs.ds.Add(room.Id(), room.Name(), room)
		if err != nil {
			return nil, fmt.Errorf("Add(%s): %s", room.Id(), err)
		}
	}

	rs.ds.Activate()
	return rs, nil
}

func (rs *RoomSet) Event(evt slack.SlackEvent) bool {
	rs.Lock()
	defer rs.Unlock()
	switch msg := evt.Data.(type) {
	case slack.AckMessage:
		for _, room := range rs.objs {
			if ok := room.Event(evt); ok {
				return true
			}
		}
		return false
	case *slack.MessageEvent:
		if r, ok := rs.objs[msg.ChannelId]; ok {
			return r.Event(evt)
		}
	}
	return false
}
