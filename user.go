// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync"

	"github.com/bpowers/slack"
)

type User struct {
	slack.User
	mu   sync.Mutex
	conn *FSConn
}

func NewUser(su slack.User, conn *FSConn) *User {
	u := new(User)
	u.User = su
	u.conn = conn

	return u
}

type userIdNode struct {
	AttrNode
}

func newUserId(parent *DirNode) (INode, error) {
	name := "id"
	n := new(userIdNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userIdNode) Update() {
	val := n.parent.priv.(*User).Id + "\n"
	n.updateCommon(val)
}

type userNameNode struct {
	AttrNode
}

func newUserName(parent *DirNode) (INode, error) {
	name := "name"
	n := new(userNameNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userNameNode) Update() {
	val := n.parent.priv.(*User).Name + "\n"
	n.updateCommon(val)
}

type userPresenceNode struct {
	AttrNode
}

func newUserPresence(parent *DirNode) (INode, error) {
	name := "presence"
	n := new(userPresenceNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userPresenceNode) Update() {
	val := n.parent.priv.(*User).Presence + "\n"
	n.updateCommon(val)
}

type userIsBotNode struct {
	AttrNode
}

func newUserIsBot(parent *DirNode) (INode, error) {
	name := "is-bot"
	n := new(userIsBotNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userIsBotNode) Update() {
	var val string
	if n.parent.priv.(*User).IsBot {
		val = "true\n"
	} else {
		val = "false\n"
	}
	n.updateCommon(val)
}

var userAttrs = []AttrFactory{
	newUserId,
	newUserName,
	newUserPresence,
	newUserIsBot,
}

func NewUserDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*User); !ok {
		return nil, fmt.Errorf("NewUserDir called w non-user: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range userAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		dir.addChild(n)
	}

	return dir, nil
}
