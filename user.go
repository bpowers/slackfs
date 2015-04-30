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
	val := n.parent.priv.(*slack.User).Id + "\n"
	n.updateCommon(val)
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
	val := n.parent.priv.(*slack.User).Name + "\n"
	n.updateCommon(val)
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
	val := n.parent.priv.(*slack.User).Presence + "\n"
	n.updateCommon(val)
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
	var val string
	if n.parent.priv.(*slack.User).IsBot {
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

func NewUserDir(parent *DirNode, u *slack.User) (*DirNode, error) {
	dir, err := NewDirNode(parent, u.Id, u)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range userAttrs {
		n, err := attrFactory(dir, nil)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		dir.addChild(n)
	}

	return dir, nil
}
