package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/nlopes/slack"

	"bazil.org/fuse"
	"golang.org/x/net/context"
)

type Channel struct {
	slack.Channel
	fs *FS
}

type Group struct {
	slack.Group
	fs *FS
}

type IdNamer interface {
	Id() string
	Name() string
}

type DirSet struct {
	dn      *DirNode // us
	parent  *DirNode
	byName  *DirNode // by-name child dir
	byId    *DirNode // by-id child dir
	objDirs map[string]*DirNode
	objSyms map[string]*SymlinkNode
}

func NewDirSet(fs *FS, parent *DirNode, name string) (ds *DirSet, err error) {
	ds = new(DirSet)
	ds.dn, err = NewDirNode(parent, name, fs)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode('%s'): %s", name, err)
	}
	ds.byName, err = NewDirNode(ds.dn, "by-name", fs)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode(by-name): %s", err)
	}
	ds.byId, err = NewDirNode(ds.dn, "by-id", fs)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode(by-id): %s", err)
	}

	ds.objDirs = make(map[string]*DirNode)
	ds.objSyms = make(map[string]*SymlinkNode)

	return
}

func (ds *DirSet) LookupId(id string) *DirNode {
	return ds.objDirs[id]
}

func (ds *DirSet) Container() *DirNode {
	return ds.byId
}

func (ds *DirSet) Add(id, name string, child *DirNode) error {
	ds.objDirs[id] = child
	s, err := NewSymlinkNode(ds.byName, name, "../by-id/"+id, child)
	if err != nil {
		return fmt.Errorf("NewSymlinkNode(%s): %s", name, err)
	}
	ds.objSyms[name] = s
	return nil
}

func (ds *DirSet) Activate() {
	for _, n := range ds.objDirs {
		n.Activate()
	}
	ds.byId.Activate()
	for _, n := range ds.objSyms {
		n.Activate()
	}
	ds.byName.Activate()
	ds.dn.Activate()
}

type FS struct {
	super *Super
	api   *slack.Slack
	ws    *slack.SlackWS
	out   chan slack.OutgoingMessage
	in    chan slack.SlackEvent
	info  *slack.Info

	users    *DirSet
	channels *DirSet
	groups   *DirSet
}

func NewFS(token string) (*FS, error) {
	api := slack.New(token)
	ws, err := api.StartRTM("", "https://slack.com")
	if err != nil {
		return nil, fmt.Errorf("StartRTM(): %s\n", err)
	}

	info := api.GetInfo()

	fs := &FS{
		api:  api,
		ws:   ws,
		out:  make(chan slack.OutgoingMessage),
		in:   make(chan slack.SlackEvent),
		info: &info,
	}

	fs.super = NewSuper()

	api.SetDebug(true)
	go ws.HandleIncomingEvents(fs.in)
	go ws.Keepalive(10 * time.Second)

	for _, c := range fs.info.Channels {
		if !c.IsMember {
			continue
		}
		fmt.Printf("%s (%d members)\n", c.Name, len(c.Members))
	}

	err = fs.init()
	if err != nil {
		return nil, fmt.Errorf("init: %s", err)
	}

	go fs.routeIncomingEvents()

	return fs, nil
}

func NewOfflineFS(infoPath string) (*FS, error) {
	buf, err := ioutil.ReadFile(infoPath)
	if err != nil {
		return nil, fmt.Errorf("ReadFile(%s): %s", infoPath, err)
	}
	var info slack.Info
	err = json.Unmarshal(buf, &info)
	if err != nil {
		return nil, fmt.Errorf("Unmarshal: %s", err)
	}

	fs := &FS{
		out:  make(chan slack.OutgoingMessage),
		in:   make(chan slack.SlackEvent),
		info: &info,
	}

	fs.super = NewSuper()

	for _, c := range info.Channels {
		if !c.IsMember {
			continue
		}
		fmt.Printf("%s (%d members)\n", c.Name, len(c.Members))
	}

	err = fs.init()
	if err != nil {
		return nil, fmt.Errorf("init: %s", err)
	}

	//go fs.routeIncomingEvents()

	return fs, nil
}

func (fs *FS) init() error {
	root := fs.super.GetRoot()

	err := fs.initUsers(root)
	if err != nil {
		return fmt.Errorf("initUsers: %s", err)
	}
	err = fs.initChannels(root)
	if err != nil {
		return fmt.Errorf("initChannels: %s", err)
	}
	err = fs.initGroups(root)
	if err != nil {
		return fmt.Errorf("initChannels: %s", err)
	}

	return nil
}

func (fs *FS) initUsers(parent *DirNode) (err error) {
	fs.users, err = NewDirSet(fs, fs.super.root, "users")
	if err != nil {
		return fmt.Errorf("NewDirSet('users'): %s", err)
	}

	userParent := fs.users.Container()
	for _, u := range fs.info.Users {
		up := new(slack.User)
		*up = u
		ud, err := NewUserDir(userParent, up)
		if err != nil {
			return fmt.Errorf("NewUserDir(%s): %s", up.Id, err)
		}
		err = fs.users.Add(u.Id, u.Name, ud)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", up.Id, err)
		}
	}

	fs.users.Activate()
	return nil
}

func (fs *FS) initChannels(parent *DirNode) (err error) {
	fs.channels, err = NewDirSet(fs, fs.super.root, "channels")
	if err != nil {
		return fmt.Errorf("NewDirSet('channels'): %s", err)
	}

	chanParent := fs.users.Container()
	for _, c := range fs.info.Channels {
		cp := new(Channel)
		cp.Channel = c
		cd, err := NewChannelDir(chanParent, cp)
		if err != nil {
			return fmt.Errorf("NewChanDir(%s): %s", cp.Id, err)
		}
		err = fs.channels.Add(c.Id, c.Name, cd)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", cp.Id, err)
		}
	}

	fs.channels.Activate()
	return nil
}

func (fs *FS) initGroups(parent *DirNode) (err error) {
	fs.groups, err = NewDirSet(fs, fs.super.root, "groups")
	if err != nil {
		return fmt.Errorf("NewDirSet('groups'): %s", err)
	}

	groupParent := fs.users.Container()
	for _, g := range fs.info.Groups {
		gp := new(Group)
		gp.Group = g
		gd, err := NewGroupDir(groupParent, gp)
		if err != nil {
			return fmt.Errorf("NewChanDir(%s): %s", gp.Id, err)
		}
		err = fs.groups.Add(g.Id, g.Name, gd)
		if err != nil {
			return fmt.Errorf("Add(%s): %s", gp.Id, err)
		}
	}

	fs.groups.Activate()
	return nil
}

func (fs *FS) GetUser(id string) (*slack.User, bool) {
	userDir := fs.users.LookupId(id)
	if userDir == nil {
		return nil, false
	}
	u, ok := userDir.priv.(*slack.User)
	return u, ok
}

func (fs *FS) routeIncomingEvents() {
	for {
		msg := <-fs.in

		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			fmt.Printf("msg\t%s\t%s\t%s\t(%#v)\n", ev.Timestamp, ev.UserId, ev.Text, ev)
		case *slack.PresenceChangeEvent:
			name := "<unknown>"
			if u, ok := fs.GetUser(ev.UserId); ok {
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

func (fs *FS) Send(txtBytes []byte, id string) error {
	txt := strings.TrimSpace(string(txtBytes))

	out := fs.ws.NewOutgoingMessage(txt, id)
	err := fs.ws.SendMessage(out)
	if err != nil {
		log.Printf("SendMessage: %s", err)
	}
	// TODO(bp) add this message to the session buffer, after we
	// get an ok
	return err
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
