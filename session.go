// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"
	"text/template"

	"github.com/bpowers/fuse"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

const defaultMsgTmpl = "{{.Timestamp}}\t{{.Username}}\t{{.Text}}\n"

var t = template.Must(template.New("msg").Parse(defaultMsgTmpl))

type CtlEventType int

const (
	WorkerStop CtlEventType = iota
)

type RoomCtlEvent struct {
	Type CtlEventType
}

type Session struct {
	sync.Cond
	mu sync.Mutex

	// everything below here must be accessed with Session.L held.
	// When any of the below are changed, Broadcast is called on
	// cond.

	initialized bool
	formatted   bytes.Buffer
}

func (s *Session) CurrLen() uint64 {
	s.L.Lock()
	for !s.initialized {
		s.Wait()
	}
	defer s.L.Unlock()
	return uint64(s.formatted.Len())
}

func (s *Session) Bytes(offset int64, size int) ([]byte, error) {
	s.L.Lock()
	for !s.initialized {
		s.Wait()
	}
	defer s.L.Unlock()
	if offset != 0 {
		log.Printf("TODO: read w/ offset not implemented yet")
		return nil, fuse.EIO
	}
	bytes := s.formatted.Bytes()
	if len(bytes) > size {
		bytes = bytes[:size]
	}
	return bytes, nil
}

// must be called with s.L held
func (s *Session) formatMsg(msg slack.Message) error {
	return t.Execute(&s.formatted, msg)
}

type historyFn func(id string, params slack.HistoryParameters) (*slack.History, error)

func (s *Session) getHistory(fn historyFn, id, lastRead string) error {
	h, err := fn(id, slack.HistoryParameters{
		Oldest:    lastRead,
		Count:     1000,
		Inclusive: true,
	})
	if err != nil {
		return fmt.Errorf("GetChannelHistory(%s): %s", id, err)
	}

	if h.HasMore {
		log.Printf("TODO: we need to page/fetch more messages")
	}

	s.L.Lock()
	defer s.L.Unlock()

	for _, msg := range h.Messages {
		err := s.formatMsg(msg)
		if err != nil {
			log.Printf("formatMsg(%#v): %s", msg, err)
		}
	}
	s.initialized = true
	s.Broadcast()

	return nil
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
