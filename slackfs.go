package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"slackfs/internal/github.com/nlopes/slack"

	//"bazil.org/fuse"
	//"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

type FS struct {
	super *Super
	api   *slack.Slack
	ws    *slack.SlackWS
	out   chan slack.OutgoingMessage
	in    chan slack.SlackEvent

	users    map[string]*slack.User
	userdirs map[string]*DirNode
}

func NewFS(token string) (*FS, error) {
	api := slack.New(token)
	ws, err := api.StartRTM("", "https://slack.com")
	if err != nil {
		return nil, fmt.Errorf("StartRTM(): %s\n", err)
	}

	fs := &FS{
		api:      api,
		ws:       ws,
		out:      make(chan slack.OutgoingMessage),
		in:       make(chan slack.SlackEvent),
		users:    make(map[string]*slack.User),
		userdirs: make(map[string]*DirNode),
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

	err = fs.initUsers(&info)
	if err != nil {
		return nil, fmt.Errorf("initUsers: %s", err)
	}
	// create root inode
	// create users, chans, dms inodes

	go fs.routeIncomingEvents()

	return fs, nil
}

func NewOfflineFS(infoPath string) (*FS, error) {
	var info slack.Info

	buf, err := ioutil.ReadFile(infoPath)
	if err != nil {
		return nil, fmt.Errorf("ReadFile(%s): %s", infoPath, err)
	}
	err = json.Unmarshal(buf, &info)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %s", err)
	}

	fs := &FS{
		out:      make(chan slack.OutgoingMessage),
		in:       make(chan slack.SlackEvent),
		users:    make(map[string]*slack.User),
		userdirs: make(map[string]*DirNode),
	}

	fs.super = NewSuper()

	for _, c := range info.Channels {
		if !c.IsMember {
			continue
		}
		fmt.Printf("%s (%d members)\n", c.Name, len(c.Members))
	}

	err = fs.initUsers(&info)
	if err != nil {
		return nil, fmt.Errorf("initUsers: %s", err)
	}
	// create root inode
	// create users, chans, dms inodes

	//go fs.routeIncomingEvents()

	return fs, nil
}

func (fs *FS) initUsers(info *slack.Info) error {
	root := fs.super.GetRoot()
	for _, u := range info.Users {
		up := new(slack.User)
		*up = u
		fs.users[u.Id] = up
		ud, err := NewUserDir(root, up)
		if err != nil {
			return fmt.Errorf("NewUserDir(%s): %s", up.Id, err)
		}
		fs.userdirs[u.Id] = ud
	}
	return nil
}

func (fs *FS) GetUser(id string) (*slack.User, bool) {
	u, ok := fs.users[id]
	return u, ok
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
