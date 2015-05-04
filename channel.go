// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/bpowers/slack"
)

type Channel struct {
	slack.Channel
	Session
}

func NewChannel(sc slack.Channel, conn *FSConn) *Channel {
	c := new(Channel)
	c.Channel = sc
	SessionInit(&c.Session, sc.Id, conn, conn.api.GetChannelHistory)

	// fetch session history in the background
	if c.IsOpen() {
		go c.FetchHistory(c.LastRead, true)
	}

	return c
}

func (c *Channel) Id() string {
	return c.Channel.Id
}

func (c *Channel) Name() string {
	return c.Channel.Name
}

func (c *Channel) IsOpen() bool {
	return c.Channel.IsMember
}

func NewChannelDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*Channel); !ok {
		return nil, fmt.Errorf("NewChannelDir called w non-chan: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range roomAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}
