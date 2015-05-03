// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"

	"github.com/nlopes/slack"
)

type IM struct {
	slack.IM
	Session
}

func NewIM(sim slack.IM, conn *FSConn) *IM {
	im := new(IM)
	im.IM = sim
	SessionInit(&im.Session, sim.Id, conn, conn.api.GetIMHistory)

	// fetch session history in the background
	if im.IsOpen() {
		go im.FetchHistory(im.LastRead, true)
	}

	return im
}

func (im *IM) Id() string {
	return im.IM.Id
}

func (im *IM) Name() string {
	u := im.conn.users.Get(im.UserId)
	if u == nil {
		log.Printf("im with unknown user: %s", im.UserId)
		return im.IM.UserId
	}
	return u.Name
}

func (im *IM) IsOpen() bool {
	return im.IM.IsOpen
}

var imAttrs = []AttrFactory{
	newSessionWrite,
	newSession,
}

func NewIMDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*IM); !ok {
		return nil, fmt.Errorf("NewIMDir called w non-im: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range imAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}