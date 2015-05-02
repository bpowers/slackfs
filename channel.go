// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/bpowers/fuse"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

type CtlEventType int

const (
	WorkerStop CtlEventType = iota
)

type RoomCtlEvent struct {
	Type CtlEventType
}

type Channel struct {
	slack.Channel
	Session

	conn *FSConn

	running uint32 // updated atomically, lock-free read
	event   chan RoomCtlEvent
}

func NewChannel(sc slack.Channel, conn *FSConn) *Channel {
	c := new(Channel)
	c.Channel = sc
	c.conn = conn
	c.L = &c.mu

	c.event = make(chan RoomCtlEvent)

	go c.work()

	return c
}

func (c *Channel) work() {
	atomic.StoreUint32(&c.running, 1)
	// we unconditionally start workers for every known channel,
	// but don't request history for channels we're not a part of.
	if c.IsOpen() {
		if err := c.getHistory(c.conn.api, c.Id(), c.LastRead); err != nil {
			log.Printf("'%s'.getHistory(): %s", c.Name(), err)
		}
	}
outer:
	for {
		ev := <-c.event
		switch ev.Type {
		case WorkerStop:
			break outer
		}
	}
	atomic.StoreUint32(&c.running, 0)
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

func (c *Channel) Event(evt slack.SlackEvent) (handled bool) {
	// TODO(bp) implement
	return false
}

type channelCtlNode struct {
	AttrNode
}

func newChannelCtl(parent *DirNode) (INode, error) {
	name := "ctl"
	n := new(channelCtlNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
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

func newChannelWrite(parent *DirNode) (INode, error) {
	name := "write"
	n := new(channelWriteNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *channelWriteNode) Update() {
}

func (n *channelWriteNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	c, ok := n.parent.priv.(*Channel)
	if !ok {
		log.Printf("priv is not channel")
		return fuse.ENOSYS
	}

	return c.conn.Send(req.Data, c.Id())
}

func newSession(parent *DirNode) (INode, error) {
	name := "session"
	n := new(SessionAttrNode)
	if err := n.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.mode = 0444
	return n, nil
}

func (an *SessionAttrNode) Activate() error {
	if an.parent == nil {
		return nil
	}

	return an.parent.addChild(an)
}

func (an *SessionAttrNode) DirentType() fuse.DirentType {
	return fuse.DT_File
}

func (an *SessionAttrNode) IsDir() bool {
	return false
}

type SessionProvider interface {
	CurrLen() uint64
	Bytes(offset int64, size int) ([]byte, error)
}

type SessionAttrNode struct {
	Node
	Mode os.FileMode
	Size int
}

func (an *SessionAttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.ino
	a.Mode = an.Mode
	a.Size = an.parent.priv.(SessionProvider).CurrLen()
}

func (an *SessionAttrNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	provider := an.parent.priv.(SessionProvider)

	frag, err := provider.Bytes(req.Offset, req.Size)
	if err != nil {
		return fmt.Errorf("GetBytes(%d, %d): %s", req.Offset, req.Size, err)
	}

	an.Size += len(frag)

	resp.Data = frag
	return nil
}

// TODO(bp) conceptually these would be better as FIFOs, but when mode
// has os.NamedPipe the writer (bash) hangs on an open() that we never
// get a fuse request for.
var channelAttrs = []AttrFactory{
	//newChannelCtl,
	newChannelWrite,
	newSession,
}

func NewChannelDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*Channel); !ok {
		return nil, fmt.Errorf("NewChannelDir called w non-chan: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range channelAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}
