package main

import (
	"fmt"
	"time"

	"slackfs/internal/github.com/nlopes/slack"

	//"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

type FS struct {
	super *Super
	api   *slack.Slack
	ws    *slack.SlackWS
	out   chan slack.OutgoingMessage
	in    chan slack.SlackEvent

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

	fs.super = NewSuper()

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

var (
	trueBytes  = []byte("true")
	falseBytes = []byte("true")
)

func readUserId(ctx context.Context, n *Node) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Id), nil
}

func readUserName(ctx context.Context, n *Node) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Name), nil
}

func readUserPresence(ctx context.Context, n *Node) ([]byte, error) {
	return []byte(n.priv.(*slack.User).Presence), nil
}

func readUserIsBot(ctx context.Context, n *Node) ([]byte, error) {
	if n.priv.(*slack.User).IsBot {
		return trueBytes, nil
	} else {
		return falseBytes, nil
	}
}

var userAttrs = []AttrType{
	{Name: "id", ReadAll: readUserId},
	{Name: "name", ReadAll: readUserName},
	{Name: "presence", ReadAll: readUserPresence},
	{Name: "is_bot", ReadAll: readUserIsBot},
}

func NewUserDir(parent *DirNode, u *slack.User) (*DirNode, error) {
	dn, err := NewDirNode(parent, u.Id, u)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode: %s", err)
	}

	for _, ua := range userAttrs {
		an, err := NewAttrNode(dn, &ua, u)
		if err != nil {
			return nil, fmt.Errorf("NewAttrNode(%#v): %s", &ua, err)
		}
		an.n.Activate()
	}

	dn.n.Activate()

	return dn, nil
}
