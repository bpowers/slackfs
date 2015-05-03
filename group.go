// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"sync/atomic"

	"github.com/bpowers/fuse"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type Group struct {
	slack.Group
	Session

	conn *FSConn

	running uint32 // updated atomically, lock-free read
	event   chan RoomCtlEvent
}

func NewGroup(sg slack.Group, conn *FSConn) *Group {
	g := new(Group)
	g.Group = sg
	g.conn = conn
	g.L = &g.mu

	g.event = make(chan RoomCtlEvent)

	go g.work()

	return g
}

func (g *Group) work() {
	atomic.StoreUint32(&g.running, 1)
	// we unconditionally start workers for every known channel,
	// but don't request history for channels we're not a part of.
	if g.IsOpen() {
		if err := g.getHistory(g.conn.api.GetGroupHistory, g.Id(), g.LastRead); err != nil {
			log.Printf("'%s'.getHistory(): %s", g.Name(), err)
		}
	}
outer:
	for {
		ev := <-g.event
		switch ev.Type {
		case WorkerStop:
			break outer
		}
	}
	atomic.StoreUint32(&g.running, 0)
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

func (g *Group) Event(evt slack.SlackEvent) (handled bool) {
	// TODO(bp) implement
	return false
}

type groupWriteNode struct {
	AttrNode
}

func newGroupWrite(parent *DirNode) (INode, error) {
	name := "write"
	n := new(groupWriteNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *groupWriteNode) Update() {
}

func (n *groupWriteNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	g, ok := n.parent.priv.(*Group)
	if !ok {
		log.Printf("priv is not group")
		return fuse.ENOSYS
	}

	return g.conn.Send(req.Data, g.Id())
}

var groupAttrs = []AttrFactory{
	newGroupWrite,
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

	// TODO(bp) session file

	return dir, nil
}
