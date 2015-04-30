// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"bazil.org/fuse"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type groupCtlNode struct {
	AttrNode
}

func newGroupCtl(parent *DirNode, priv interface{}) (INode, error) {
	name := "ctl"
	n := new(groupCtlNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
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

func newGroupWrite(parent *DirNode, priv interface{}) (INode, error) {
	name := "write"
	n := new(groupWriteNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *groupWriteNode) Update() {
}

func (n *groupWriteNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	g, ok := n.priv.(*Group)
	if !ok {
		log.Printf("priv is not group")
		return fuse.ENOSYS
	}

	return g.fs.Send(req.Data, g.Id)
}

type Group struct {
	slack.Group
	fs *FSConn
}

var groupAttrs = []AttrFactory{
	newGroupCtl,
	newGroupWrite,
}

func NewGroupDir(parent *DirNode, g *Group) (*DirNode, error) {
	dir, err := NewDirNode(parent, g.Id, g)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range groupAttrs {
		n, err := attrFactory(dir, g)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	// TODO(bp) session file

	return dir, nil
}
