// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/nlopes/slack"
)

var (
	trueBytes  = []byte("true\n")
	falseBytes = []byte("false\n")
)

func readUserIdLen(n *AttrNode) int {
	return len(n.priv.(*slack.User).Id) + 1
}

func readUserNameLen(n *AttrNode) int {
	return len(n.priv.(*slack.User).Name) + 1
}

func readUserPresenceLen(n *AttrNode) int {
	return len(n.priv.(*slack.User).Presence) + 1
}

func readUserIsBotLen(n *AttrNode) int {
	if n.priv.(*slack.User).IsBot {
		return len("true") + 1
	} else {
		return len("false") + 1
	}
}

func readUserId(ctx context.Context, n *AttrNode) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Id + "\n"), nil
}

func readUserName(ctx context.Context, n *AttrNode) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Name + "\n"), nil
}

func readUserPresence(ctx context.Context, n *AttrNode) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Presence + "\n"), nil
}

func readUserIsBot(ctx context.Context, n *AttrNode) ([]byte, error) {
	if n.priv.(*slack.User).IsBot {
		return trueBytes, nil
	} else {
		return falseBytes, nil
	}
}

var userAttrs = []AttrType{
	{Name: "id", ReadLen: readUserIdLen, ReadAll: readUserId},
	{Name: "name", ReadLen: readUserNameLen, ReadAll: readUserName},
	{Name: "presence", ReadLen: readUserPresenceLen, ReadAll: readUserPresence},
	{Name: "is_bot", ReadLen: readUserIsBotLen, ReadAll: readUserIsBot},
}

func NewUserDir(parent *DirNode, u *slack.User) (*DirNode, error) {
	dn, err := NewDirNode(parent, u.Id, u)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for i, _ := range userAttrs {
		an, err := NewAttrNode(dn, &userAttrs[i], u)
		if err != nil {
			return nil, fmt.Errorf("NewAttrNode(%#v): %s", &userAttrs[i], err)
		}
		an.Activate()
	}

	dn.Activate()

	return dn, nil
}
