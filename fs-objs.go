// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slackfs

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/bpowers/fuse"
	"github.com/bpowers/fuse/fs"
	"golang.org/x/net/context"
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

type Super struct {
	seq  Sequence
	root *DirNode
	// TODO(bp) locks
}

func (s *Super) Init() {
	s.seq.Init()
	// TODO(bp) init root node?
}

func (s *Super) NextInodeNum() uint64 {
	return s.seq.Next()
}

func (s *Super) GetRoot() *DirNode {
	return s.root
}

func (s *Super) Root() (fs.Node, error) {
	return s.root, nil
}

type Node struct {
	super  *Super
	parent *DirNode
	name   string

	// usually a link back to the struct embedding this node
	priv interface{}

	ino  uint64
	mode os.FileMode
}

func (n *Node) Name() string {
	return n.name
}

func (n *Node) Init(parent *DirNode, name string, priv interface{}) error {
	if parent == nil {
		return fmt.Errorf("nil parent")
	}
	if name == "" {
		return fmt.Errorf("empty name")
	}
	n.super = parent.super
	n.parent = parent
	n.name = name

	n.priv = priv

	n.ino = parent.super.NextInodeNum()

	return nil
}

func (n *Node) INum() uint64 {
	return n.ino
}

func (n *Node) Parent() *DirNode {
	return n.parent
}

func (n *DirNode) Dirent() fuse.Dirent {
	return fuse.Dirent{n.ino, fuse.DT_Dir, n.name}
}

func (n *SymlinkNode) Dirent() fuse.Dirent {
	return fuse.Dirent{n.ino, fuse.DT_Link, n.name}
}

func (n *AttrNode) Dirent() fuse.Dirent {
	return fuse.Dirent{n.ino, fuse.DT_File, n.name}
}

type Updater interface {
	Update()
}

type INode interface {
	fs.Node
	Dirent() fuse.Dirent
	IsDir() bool
	Activate() error
	Name() string
	INum() uint64
	Parent() *DirNode
	//Update() ?
}

func (dn *DirNode) IsDir() bool {
	return true
}

func (sn *SymlinkNode) IsDir() bool {
	return false
}

func (an *AttrNode) IsDir() bool {
	return false
}

func (dn *DirNode) Activate() error {
	if dn.parent == nil {
		return nil
	}

	return dn.parent.addChild(dn)
}

func (sn *SymlinkNode) Activate() error {
	if sn.parent == nil {
		return nil
	}

	return sn.parent.addChild(sn)
}

func (an *AttrNode) Activate() error {
	if an.parent == nil {
		return nil
	}

	return an.parent.addChild(an)
}

type DirNode struct {
	Node

	// lock only for adding/removing children
	mu sync.Mutex

	childmap map[string]INode
	children []INode
}

func (dn *DirNode) addChild(child INode) error {
	dn.childmap[child.Name()] = child
	dn.children = append(dn.children, child)
	return nil
}

func (dn *DirNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if n, ok := dn.childmap[name]; ok {
		return n, nil
	} else {
		return nil, fuse.ENOENT
	}
}

func (dn *DirNode) Attr(a *fuse.Attr) {
	a.Inode = dn.ino
	a.Mode = dn.mode

	// linkcount for dirs is 2 + n subdirs
	a.Nlink = 2
	for _, n := range dn.children {
		if n.IsDir() {
			a.Nlink++
		}
	}
}

func (dn *DirNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dents := make([]fuse.Dirent, 0, len(dn.children))
	for _, child := range dn.children {
		dents = append(dents, child.Dirent())
	}
	return dents, nil
}

type SymlinkNode struct {
	Node
	path   string
	target INode
}

func (sn *SymlinkNode) Attr(a *fuse.Attr) {
	a.Inode = sn.ino
	a.Mode = os.ModeSymlink | sn.mode
}

func (sn *SymlinkNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return sn.path, nil
}

type AttrFactory func(parent *DirNode) (INode, error)

type AttrNode struct {
	Node

	size    uint64
	content atomic.Value // []byte
}

func (an *AttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.ino
	a.Mode = an.mode
	a.Size = an.size
}

func (n *AttrNode) updateCommon(val string) {
	size := len(val)
	atomic.StoreUint64(&n.size, uint64(size))

	var content *[]byte
	if size != 0 {
		contentSlice := []byte(val)
		content = &contentSlice
	}
	n.content.Store(content)
}

func (an *AttrNode) ReadAll(ctx context.Context) ([]byte, error) {
	// if content is nil, it means we are write-only.
	content := an.content.Load()
	if content == nil || content.(*[]byte) == nil {
		return nil, fuse.ENOSYS
	}
	return *content.(*[]byte), nil
}

func NewSuper() *Super {
	super := new(Super)
	super.Init()

	root := new(DirNode)

	// FIXME(bp) open coded from Node.Init
	root.super = super
	root.ino = super.NextInodeNum()

	// FIXME(bp) open coded from NewDirNode
	root.childmap = make(map[string]INode)
	root.children = make([]INode, 0, 8)
	root.mode = os.ModeDir | 0555

	super.root = root

	return super
}

func NewDirNode(parent *DirNode, name string, priv interface{}) (*DirNode, error) {
	dn := new(DirNode)
	err := dn.Node.Init(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s', %#v): %s", name, priv, err)
	}
	dn.childmap = make(map[string]INode)
	dn.children = make([]INode, 0)

	dn.mode = os.ModeDir | 0555

	return dn, nil
}

func findCommonAncestor(a, b INode) (INode, error) {
	marked := make(map[uint64]struct{})
	queue := make(chan INode, 2)

	marked[a.INum()] = struct{}{}
	marked[b.INum()] = struct{}{}

	if parent := a.Parent(); parent != nil {
		queue <- parent
	}
	if parent := b.Parent(); parent != nil {
		queue <- parent
	}

outer:
	for {
		select {
		case node := <-queue:
			if _, ok := marked[node.INum()]; ok {
				return node, nil
			}
			marked[node.INum()] = struct{}{}
			if parent := node.Parent(); parent != nil {
				queue <- parent
			}
		default:
			break outer
		}
	}

	return nil, fmt.Errorf("no common ancestor")
}

func relativeSymlinkPath(ancestor, parent, target INode) string {
	var buf bytes.Buffer

	if target == parent {
		return "."
	}

	for n := parent; n != ancestor; n = n.Parent() {
		buf.WriteString("../")
	}

	targetDirs := make([]INode, 0, 4)
	for n := target; n != ancestor; n = n.Parent() {
		targetDirs = append(targetDirs, n)
	}
	if len(targetDirs) > 0 {
		for i := len(targetDirs) - 1; i >= 0; i-- {
			buf.WriteString(targetDirs[i].Name())
			buf.WriteByte('/')
		}
	}
	// in all possible cases we've added a trailing slash that
	// should be removed
	result := buf.String()
	return result[:len(result)-1]
}

func NewSymlinkNode(parent *DirNode, name string, target INode) (*SymlinkNode, error) {
	sn := new(SymlinkNode)
	err := sn.Node.Init(parent, name, nil)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s'): %s", name, err)
	}

	// XXX: locking?
	ancestor, err := findCommonAncestor(target, parent)
	if err != nil {
		return nil, fmt.Errorf("findCommonAncestor: %s", err)
	}

	sn.path = relativeSymlinkPath(ancestor, parent, target)
	sn.target = target
	sn.mode = os.ModeSymlink | 0777

	return sn, nil
}

func NewAttrNode(parent *DirNode, name string, priv interface{}) (*AttrNode, error) {
	an := new(AttrNode)
	err := an.Node.Init(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s', %#v): %s", name, priv, err)
	}
	return an, nil
}
