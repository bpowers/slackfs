package main

import (
	"fmt"
	"time"

	"slackfs/internal/github.com/nlopes/slack"
	
	//"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type Sequence struct {
	n chan uint64
}

func (s *Sequence) Init() {
	s.n = make(chan uint64)
	go s.gen()
}

func (s *Sequence) Close() {
	close(s.n)
}

func (s *Sequence) gen() {
	for i := uint64(1); ; i++ {
		s.n <- i
	}
}

func (s *Sequence) Next() uint64 {
	return <-s.n
}

type FS struct {
	seq Sequence
	api *slack.Slack
	ws  *slack.SlackWS
	out chan slack.OutgoingMessage
	in  chan slack.SlackEvent

	users map[string]*slack.User
}

func NewFS(token string) (*FS, error) {
	api := slack.New(token)
	ws, err := api.StartRTM("", "https://slack.com")
	if err != nil {
		return nil, fmt.Errorf("StartRTM(): %s\n", err)
	}

	fs := &FS{
		api:   api,
		ws:    ws,
		out:   make(chan slack.OutgoingMessage),
		in:    make(chan slack.SlackEvent),
		users: make(map[string]*slack.User),
	}

	fs.seq.Init()

	api.SetDebug(true)
	go ws.HandleIncomingEvents(fs.in)
	go ws.Keepalive(10 * time.Second)

	info := api.GetInfo()

	for _, c := range info.Channels {
		if !c.IsMember {
			continue
		}
		fmt.Printf("%s (%d members)\n", c.Name, len(c.Members))
	}

	fs.initUsers(&info)

	// create root inode
	// create users, chans, dms inodes

	go fs.routeIncomingEvents()

	return fs, nil
}

func (fs *FS) NextInodeNum() uint64 {
	return fs.seq.Next()
}

func (fs *FS) initUsers(info *slack.Info) {
	for _, u := range info.Users {
		up := new(slack.User)
		*up = u
		fs.users[u.Id] = up
	}
}

func (fs *FS) GetUser(id string) (*slack.User, bool) {
	u, ok := fs.users[id]
	return u, ok
}

func (fs *FS) Root() (fs.Node, error) {
	return nil, fmt.Errorf("not implemented")
}

func (fs *FS) routeIncomingEvents() {
	for {
		msg := <-fs.in

		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			fmt.Printf("msg\t%s\t%s\t%s\n", ev.Timestamp, ev.UserId, ev.Text)
		case *slack.PresenceChangeEvent:
			name := "<unknown>"
			if u, ok := fs.users[ev.UserId]; ok {
				name = u.Name
			}
			fmt.Printf("presence\t%s\t%s\n", name, ev.Presence)
		case *slack.SlackWSError:
			fmt.Printf("err: %s\n", ev)
		}
	}
}
