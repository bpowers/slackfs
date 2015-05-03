// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/nlopes/slack"
)

type Group struct {
	slack.Group
	Session
}

func NewGroup(sg slack.Group, conn *FSConn) *Group {
	g := new(Group)
	g.Group = sg
	SessionInit(&g.Session, sg.Id, conn, conn.api.GetGroupHistory)

	// fetch session history in the background
	if g.IsOpen() {
		go g.FetchHistory(g.LastRead, true)
	}

	return g
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

var groupAttrs = []AttrFactory{
	newSessionWrite,
	newSession,
}

func NewGroupDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*Group); !ok {
		return nil, fmt.Errorf("NewGroupDir called w non-group: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range groupAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}
