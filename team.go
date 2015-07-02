// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sync"

	"github.com/bpowers/slack"
)

type Team struct {
	slack.Team
	mu   sync.Mutex
	conn *FSConn
}

func NewTeam(st *slack.Team, conn *FSConn) *Team {
	t := new(Team)
	t.Team = *st
	t.conn = conn

	return t
}

type teamIdNode struct {
	AttrNode
}

func newTeamId(parent *DirNode) (INode, error) {
	name := "id"
	n := new(teamIdNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *teamIdNode) Update() {
	val := n.parent.priv.(*Team).Id + "\n"
	n.updateCommon(val)
}

type teamNameNode struct {
	AttrNode
}

func newTeamName(parent *DirNode) (INode, error) {
	name := "name"
	n := new(teamNameNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0444
	return n, nil
}

func (n *teamNameNode) Update() {
	val := n.parent.priv.(*Team).Name + "\n"
	n.updateCommon(val)
}

var teamAttrs = []AttrFactory{
	newTeamId,
	newTeamName,
}

func NewTeamDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*Team); !ok {
		return nil, fmt.Errorf("NewTeamDir called w non-team: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range teamAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		dir.addChild(n)
	}

	return dir, nil
}
