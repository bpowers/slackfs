// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/nlopes/slack"
)

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
var chanAttrs = []AttrType{
	{Name: "ctl", Write: writeChanCtl},
	{Name: "write", Write: writeChanWrite},
}

func NewChannelDir(parent *DirNode, ch *Channel) (*DirNode, error) {
	chn, err := NewDirNode(parent, ch.Id, ch)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for i, _ := range chanAttrs {
		an, err := NewAttrNode(chn, &chanAttrs[i], ch)
		if err != nil {
			return nil, fmt.Errorf("NewAttrNode(%#v): %s", &chanAttrs[i], err)
		}
		an.Activate()
	}

	// session file

	chn.Activate()

	return chn, nil
}
