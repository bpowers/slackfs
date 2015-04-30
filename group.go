// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/nlopes/slack"
)

func writeGroupCtl(ctx context.Context, an *AttrNode, off int64, msg []byte) error {
	log.Printf("ctl: %s", string(msg))
	return nil
}

func writeGroupWrite(ctx context.Context, n *AttrNode, off int64, msg []byte) error {
	g, ok := n.priv.(*Group)
	if !ok {
		log.Printf("priv is not group")
		return fuse.ENOSYS
	}

	return g.fs.Send(msg, g.Id)
}

var groupAttrs = []AttrType{
	{Name: "ctl", Write: writeGroupCtl},
	{Name: "write", Write: writeGroupWrite},
}

func NewGroupDir(parent *DirNode, g *Group) (*DirNode, error) {
	gn, err := NewDirNode(parent, g.Id, g)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for i, _ := range groupAttrs {
		an, err := NewAttrNode(gn, &groupAttrs[i], g)
		if err != nil {
			return nil, fmt.Errorf("NewAttrNode(%#v): %s", &chanAttrs[i], err)
		}
		an.Activate()
	}

	// session file

	gn.Activate()

	return gn, nil
}
