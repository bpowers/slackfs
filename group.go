// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slackfs

import (
	"fmt"

	"github.com/bpowers/slack"
)

type Group struct {
	slack.Group
	Session
}

func NewGroup(sg slack.Group, conn *FSConn) *Group {
	g := new(Group)
	g.Group = sg
	g.Session.Init(g, conn, conn.api.GetGroupHistory)

	return g
}

func (g *Group) BaseChannel() *slack.BaseChannel {
	return &g.Group.BaseChannel
}

func (g *Group) Id() string {
	return g.Group.Id
}

func (g *Group) Name() string {
	return g.Group.Name
}

func (g *Group) IsOpen() bool {
	return g.Group.IsOpen
}

func NewGroupDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*Group); !ok {
		return nil, fmt.Errorf("NewGroupDir called w non-group: %#v", priv)
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
