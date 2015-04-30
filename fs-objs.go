// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
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

func (n *Node) Dirent() fuse.Dirent {
	return fuse.Dirent{n.ino, 0, n.name}
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

func (dn *DirNode) DirentType() fuse.DirentType {
	return fuse.DT_Dir
}

func (sn *SymlinkNode) DirentType() fuse.DirentType {
	return fuse.DT_Link
}

func (an *AttrNode) DirentType() fuse.DirentType {
	return fuse.DT_File
}

type INode interface {
	fs.Node
	Dirent() fuse.Dirent
	DirentType() fuse.DirentType
	IsDir() bool
	Activate() error
	Name() string
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

	// lock for attribute writes/updates.  Reads are lock-free.
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
		dent := child.Dirent()
		dent.Type = child.DirentType()
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

type AttrFactory func(parent *DirNode, priv interface{}) (INode, error)

type AttrNode struct {
	Node
	Mode os.FileMode

	// size and content are derived from val, val must be updated
	// with mu held.
	val string

	size    uint64
	content atomic.Value // []byte
}

func (an *AttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.ino
	a.Mode = an.Mode
	a.Size = an.size
}

// must be called with mu held
func (n *AttrNode) updateCommon() {
	size := len(n.val)
	atomic.StoreUint64(&n.size, uint64(size))

	var content *[]byte
	if size != 0 {
		contentSlice := []byte(n.val)
		content = &contentSlice
	}
	n.content.Store(content)
}

func (an *AttrNode) ReadAll(ctx context.Context) ([]byte, error) {
	// if content is nil, it means we are write-only.
	content := an.content.Load().(*[]byte)
	if content == nil {
		return nil, fuse.ENOSYS
	}
	return *content, nil
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

func NewSymlinkNode(parent *DirNode, name string, targetPath string, target INode) (*SymlinkNode, error) {
	sn := new(SymlinkNode)
	err := sn.Node.Init(parent, name, nil)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s'): %s", name, err)
	}
	sn.path = targetPath
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
