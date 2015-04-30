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
	"time"

	"github.com/nlopes/slack"

	"bazil.org/fuse"
	"golang.org/x/net/context"
)

type IdNamer interface {
	Id() string
	Name() string
}

type FSConn struct {
	super *Super

	api  *slack.Slack
	ws   *slack.SlackWS
	in   chan slack.SlackEvent
	info *slack.Info

	users    *DirSet
	channels *DirSet
	groups   *DirSet
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
		conn.ws, err = conn.api.StartRTM("", "https://slack.com")
		if err != nil {
			return nil, fmt.Errorf("StartRTM(): %s\n", err)
		}
		info = conn.api.GetInfo()
	}

	//conn.api.SetDebug(true)

	conn.info = &info
	conn.in = make(chan slack.SlackEvent)
	conn.super = NewSuper()

	root := conn.super.GetRoot()

	err = conn.initUsers(root)
	if err != nil {
		return nil, fmt.Errorf("initUsers: %s", err)
	}
	err = conn.initChannels(root)
	if err != nil {
		return nil, fmt.Errorf("initChannels: %s", err)
	}
	err = conn.initGroups(root)
	if err != nil {
		return nil, fmt.Errorf("initChannels: %s", err)
	}

	go conn.ws.HandleIncomingEvents(conn.in)
	go conn.ws.Keepalive(10 * time.Second)
	go conn.routeIncomingEvents()

	return conn, nil
}

func NewFSConn(token string) (*FSConn, error) {
	return newFSConn(token, "")
}

func NewOfflineFSConn(infoPath string) (*FSConn, error) {
	return newFSConn("", infoPath)
}

func (fs *FSConn) initUsers(parent *DirNode) (err error) {
	fs.users, err = NewDirSet(fs.super.root, "users", fs)
	if err != nil {
		return fmt.Errorf("NewDirSet('users'): %s", err)
	}

	userParent := fs.users.Container()
	for _, u := range fs.info.Users {
		up := new(slack.User)
		*up = u
		ud, err := NewUserDir(userParent, up)
		if err != nil {
			return fmt.Errorf("NewUserDir(%s): %s", up.Id, err)
		}
		err = fs.users.Add(u.Id, u.Name, ud)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", up.Id, err)
		}
	}

	fs.users.Activate()
	return nil
}

func (fs *FSConn) initChannels(parent *DirNode) (err error) {
	fs.channels, err = NewDirSet(fs.super.root, "channels", fs)
	if err != nil {
		return fmt.Errorf("NewDirSet('channels'): %s", err)
	}

	chanParent := fs.users.Container()
	for _, c := range fs.info.Channels {
		cp := new(Channel)
		cp.Channel = c
		cd, err := NewChannelDir(chanParent, cp)
		if err != nil {
			return fmt.Errorf("NewChanDir(%s): %s", cp.Id, err)
		}
		err = fs.channels.Add(c.Id, c.Name, cd)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", cp.Id, err)
		}
	}

	fs.channels.Activate()
	return nil
}

func (fs *FSConn) initGroups(parent *DirNode) (err error) {
	fs.groups, err = NewDirSet(fs.super.root, "groups", fs)
	if err != nil {
		return fmt.Errorf("NewDirSet('groups'): %s", err)
	}

	groupParent := fs.users.Container()
	for _, g := range fs.info.Groups {
		gp := new(Group)
		gp.Group = g
		gd, err := NewGroupDir(groupParent, gp)
		if err != nil {
			return fmt.Errorf("NewChanDir(%s): %s", gp.Id, err)
		}
		err = fs.groups.Add(g.Id, g.Name, gd)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", gp.Id, err)
		}
	}

	fs.groups.Activate()
	return nil
}

func (fs *FSConn) GetUser(id string) (*slack.User, bool) {
	userDir := fs.users.LookupId(id)
	if userDir == nil {
		return nil, false
	}
	u, ok := userDir.priv.(*slack.User)
	return u, ok
}

func (fs *FSConn) routeIncomingEvents() {
	for {
		msg := <-fs.in

		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			fmt.Printf("msg\t%s\t%s\t%s\n", ev.Timestamp, ev.UserId, ev.Text)
		case *slack.PresenceChangeEvent:
			name := "<unknown>"
			if u, ok := fs.GetUser(ev.UserId); ok {
				name = u.Name
			}
			fmt.Printf("presence\t%s\t%s\n", name, ev.Presence)
		case *slack.SlackWSError:
			fmt.Printf("err: %s\n", ev)
		}
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
