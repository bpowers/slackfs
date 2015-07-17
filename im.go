// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slackfs

import (
	"fmt"

	"github.com/bpowers/slack"
)

type IM struct {
	slack.IM
	Session

	dir *DirNode
}

func NewIM(sim slack.IM, conn *FSConn) *IM {
	im := new(IM)
	im.IM = sim
	im.Session.Init(im, conn, conn.api.GetIMHistory)

	return im
}

func (im *IM) BaseChannel() *slack.BaseChannel {
	return &im.IM.BaseChannel
}

func (im *IM) Id() string {
	return im.IM.Id
}

func (im *IM) Name() string {
	u := im.conn.users.Get(im.UserId)
	if u == nil {
		return im.IM.UserId
	}
	return u.Name
}

func (im *IM) IsOpen() bool {
	return im.IM.IsOpen
}

func NewIMDir(parent *DirNode, id string, priv interface{}) (*DirNode, error) {
	if _, ok := priv.(*IM); !ok {
		return nil, fmt.Errorf("NewIMDir called w non-im: %#v", priv)
	}

	dir, err := NewDirNode(parent, id, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, attrFactory := range roomAttrs {
		n, err := attrFactory(dir)
		if err != nil {
			return nil, fmt.Errorf("attrFactory: %s", err)
		}
		n.Activate()
	}

	return dir, nil
}
