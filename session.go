// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"text/template"

	"github.com/bpowers/fuse"
	"github.com/nlopes/slack"
)

const defaultMsgTmpl = "{{.Timestamp}}\t{{.Username}}\t{{.Text}}\n"

var t = template.Must(template.New("msg").Parse(defaultMsgTmpl))

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

func (s *Session) getHistory(api *slack.Slack, id, lastRead string) error {
	h, err := api.GetChannelHistory(id, slack.HistoryParameters{
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
