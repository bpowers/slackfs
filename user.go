// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/nlopes/slack"
)

type userIdNode struct {
	AttrNode
}

func newUserId(parent *DirNode, priv interface{}) (INode, error) {
	name := "id"
	n := new(userIdNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userIdNode) Update() {
	n.parent.mu.Lock()
	defer n.parent.mu.Unlock()

	n.val = n.priv.(*slack.User).Id + "\n"
	n.updateCommon()
}

type userNameNode struct {
	AttrNode
}

func newUserName(parent *DirNode, priv interface{}) (INode, error) {
	name := "name"
	n := new(userNameNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userNameNode) Update() {
	n.parent.mu.Lock()
	defer n.parent.mu.Unlock()

	n.val = n.priv.(*slack.User).Name + "\n"
	n.updateCommon()
}

type userPresenceNode struct {
	AttrNode
}

func newUserPresence(parent *DirNode, priv interface{}) (INode, error) {
	name := "presence"
	n := new(userPresenceNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userPresenceNode) Update() {
	n.parent.mu.Lock()
	defer n.parent.mu.Unlock()

	n.val = n.priv.(*slack.User).Presence + "\n"
	n.updateCommon()
}

type userIsBotNode struct {
	AttrNode
}

func newUserIsBot(parent *DirNode, priv interface{}) (INode, error) {
	name := "is-bot"
	n := new(userIsBotNode)
	if err := n.AttrNode.Node.Init(parent, name, priv); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *userIsBotNode) Update() {
	n.parent.mu.Lock()
	defer n.parent.mu.Unlock()

	if n.priv.(*slack.User).IsBot {
		n.val = "true\n"
	} else {
		n.val = "false\n"
	}
	n.updateCommon()
}

var userAttrs = []AttrFactory{
	newUserId,
	newUserName,
	newUserPresence,
	newUserIsBot,
}

func NewUserDir(parent *DirNode, u *slack.User) (*DirNode, error) {
	dir, err := NewDirNode(parent, u.Id, u)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range userAttrs {
		n, err := attrFactory(dir, u)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}
