// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"github.com/bpowers/fuse"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type groupCtlNode struct {
	AttrNode
}

func newGroupCtl(parent *DirNode) (INode, error) {
	name := "ctl"
	n := new(groupCtlNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *groupCtlNode) Update() {
}

func (n *groupCtlNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	log.Printf("ctl: %s", string(req.Data))
	return nil
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

type Group struct {
	slack.Group
	conn *FSConn
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

var groupAttrs = []AttrFactory{
	newGroupCtl,
	newGroupWrite,
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
