// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/bpowers/fuse"
	"github.com/bpowers/slack"
	"golang.org/x/net/context"
)

const defaultMsgTmpl = "{{ts .Timestamp \"Jan 02 15:04:05\"}}\t{{username .}}\t{{fmt .Text}}\n"

type msgSlice []slack.Message

func (p msgSlice) Len() int           { return len(p) }
func (p msgSlice) Less(i, j int) bool { return p[i].Timestamp < p[j].Timestamp }
func (p msgSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

type HistoryFn func(id string, params slack.HistoryParameters) (*slack.History, error)

type Session struct {
	// set in SessionInit, immutable after
	history HistoryFn
	id      string
	conn    *FSConn
	fns     template.FuncMap

	sync.Cond
	mu sync.Mutex

	// everything below here must be accessed with Session.L held.

	acks map[int]struct{} // waiting for websocket acks

	// When any of the below are changed, Broadcast is called on
	// cond.

	initialized bool
	formatted   bytes.Buffer
	newestTs    string // most recent timestamp
}

func SessionInit(s *Session, id string, conn *FSConn, history HistoryFn) {
	s.L = &s.mu
	s.history = history
	s.id = id
	s.conn = conn
	s.acks = make(map[int]struct{})

	s.fns = template.FuncMap{
		"username": func(msg *slack.Message) (string, error) {
			u := s.conn.users.Get(msg.UserId)
			if u == nil {
				return fmt.Sprintf("<unknown|%s>", msg.UserId), nil
			}
			return u.Name, nil
		},
		"ts": func(ts, layout string) (string, error) {
			secs, err := strconv.ParseFloat(ts, 64)
			if err != nil {
				log.Printf("ParseFloat(%s): %s", ts, err)
				return ts, nil
			}
			sec := int64(secs)
			nsec := int64(1000000000 * (secs - math.Floor(secs)))
			t := time.Unix(sec, nsec)
			return t.Format(layout), nil
		},
		"fmt": func(txt string) (string, error) {
			return txt, nil
		},
	}
}

func (s *Session) CurrLen() uint64 {
	s.L.Lock()
	defer s.L.Unlock()
	for !s.initialized {
		s.Wait()
	}
	return uint64(s.formatted.Len())
}

func (s *Session) Bytes(offset int64, size int) ([]byte, error) {
	s.L.Lock()
	defer s.L.Unlock()
	for !s.initialized {
		s.Wait()
	}
	bytes := s.formatted.Bytes()
	if offset >= int64(len(bytes)) {
		log.Printf("TODO: offset (%d) > bytes (%s)", offset, s.id)
		return nil, fuse.EIO
	}
	bytes = bytes[offset:]
	if len(bytes) > size {
		bytes = bytes[:size]
	}
	return bytes, nil
}

func (s *Session) Write(msg []byte) error {
	msg = bytes.TrimSpace(msg)
	id := s.id
	out := s.conn.ws.NewOutgoingMessage(string(msg), id)

	// record our websocket-message ID so that we know what to do
	// when the server acknowledges receipt
	s.L.Lock()
	s.acks[out.Id] = struct{}{}
	s.L.Unlock()

	err := s.conn.ws.SendMessage(out)
	if err != nil {
		log.Printf("SendMessage: %s", err)
	}

	return nil
}

func (s *Session) Event(evt slack.SlackEvent) bool {
	switch msg := evt.Data.(type) {
	case slack.AckMessage:
		s.L.Lock()
		_, ok := s.acks[msg.ReplyTo]
		delete(s.acks, msg.ReplyTo)
		s.L.Unlock()
		if !ok {
			return false
		}
		if err := s.FetchHistory(msg.Timestamp, true); err != nil {
			log.Printf("'%s'.FetchHistory() 2: %s", s.id, err)
		}
		return true

	case *slack.MessageEvent:
		if msg.ChannelId != s.id {
			log.Printf("error: bad routing on %s for %#v", s.id, msg)
			return false
		}
		s.addMessage((*slack.Message)(msg))
		return true
	}

	return false
}

// must be called with s.L held
func (s *Session) formatMsg(msg *slack.Message) error {
	t := template.Must(template.New("msg").Funcs(s.fns).Parse(defaultMsgTmpl))
	return t.Execute(&s.formatted, msg)
}

func (s *Session) FetchHistory(oldest string, inclusive bool) error {
	h, err := s.history(s.id, slack.HistoryParameters{
		Oldest:    oldest,
		Count:     1000,
		Inclusive: inclusive,
	})
	if err != nil {
		return fmt.Errorf("GetHistory(%s): %s", s.id, err)
	}

	if h.HasMore {
		log.Printf("TODO: we need to page/fetch more messages")
	}

	sort.Sort(msgSlice(h.Messages))

	s.L.Lock()
	defer s.L.Unlock()

	for _, msg := range h.Messages {
		err := s.formatMsg(&msg)
		if err != nil {
			log.Printf("formatMsg(%#v): %s", msg, err)
		}
	}
	s.newestTs = h.Messages[len(h.Messages)-1].Timestamp
	s.initialized = true

	s.Broadcast()

	return nil
}

func (s *Session) addMessage(msg *slack.Message) error {
	s.L.Lock()
	defer s.L.Unlock()
	// don't add messages from the websocket until after we've
	// initialized history.
	for !s.initialized {
		log.Printf("waiting to init before recording msg %s", msg.Text)
		s.Wait()
	}
	if msg.Timestamp <= s.newestTs {
		log.Printf("dropping WS message %s (%s) because it is too old", msg.Timestamp, msg.Text)
		return nil
	}

	err := s.formatMsg(msg)
	if err != nil {
		log.Printf("formatMsg(%#v): %s", msg, err)
		return nil
	}
	s.newestTs = msg.Timestamp

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

type SessionWriter interface {
	Write([]byte) error
}

type SessionAttrNode struct {
	Node
	Size int
}

func (an *SessionAttrNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	resp.AttrValid = 200 * time.Millisecond
	an.Attr(&resp.Attr)
	return nil
}

func (an *SessionAttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.ino
	a.Mode = an.mode
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

type sessionWriteNode struct {
	AttrNode
}

func newSessionWrite(parent *DirNode) (INode, error) {
	name := "write"
	n := new(sessionWriteNode)
	if err := n.AttrNode.Node.Init(parent, name, nil); err != nil {
		return nil, fmt.Errorf("node.Init('%s': %s", name, err)
	}
	n.Update()
	n.mode = 0222
	return n, nil
}

func (n *sessionWriteNode) Update() {
}

func (n *sessionWriteNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	g, ok := n.parent.priv.(SessionWriter)
	if !ok {
		log.Printf("priv is not SessionWriter")
		return fuse.ENOSYS
	}

	resp.Size = len(req.Data)
	go g.Write(req.Data)

	return nil
}

func (n *sessionWriteNode) Activate() error {
	if n.parent == nil {
		return nil
	}

	return n.parent.addChild(n)
}
