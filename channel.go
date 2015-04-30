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

type channelCtlNode struct {
	AttrNode
}

func newChannelCtl(parent *DirNode, priv interface{}) (INode, error) {
	name := "ctl"
	n := new(channelCtlNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *channelCtlNode) Update() {
}

func (n *channelCtlNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	log.Printf("ctl: %s", string(req.Data))
	return nil
}

type channelWriteNode struct {
	AttrNode
}

func newChannelWrite(parent *DirNode, priv interface{}) (INode, error) {
	name := "write"
	n := new(channelWriteNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *channelWriteNode) Update() {
}

func (n *channelWriteNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	g, ok := n.priv.(*Channel)
	if !ok {
		log.Printf("priv is not channel")
		return fuse.ENOSYS
	}

	return g.fs.Send(req.Data, g.Id)
}

type Channel struct {
	slack.Channel
	fs *FSConn
}

func writeChanCtl(ctx context.Context, an *AttrNode, off int64, msg []byte) error {
	log.Printf("ctl: %s", string(msg))
	return nil
}

func writeChanWrite(ctx context.Context, n *AttrNode, off int64, msg []byte) error {
	ch, ok := n.priv.(*Channel)
	if !ok {
		log.Printf("priv is not chan")
		return fuse.ENOSYS
	}

	return ch.fs.Send(msg, ch.Id)
}

// TODO(bp) conceptually these would be better as FIFOs, but when mode
// has os.NamedPipe the writer (bash) hangs on an open() that we never
// get a fuse request for.
var channelAttrs = []AttrFactory{
	newChannelCtl,
	newChannelWrite,
}

func NewChannelDir(parent *DirNode, g *Channel) (*DirNode, error) {
	dir, err := NewDirNode(parent, g.Id, g)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range channelAttrs {
		n, err := attrFactory(dir, g)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	// TODO(bp) session file

	return dir, nil
}
