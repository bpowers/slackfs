package main

import (
	"fmt"
	"os"

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

type AttrType struct {
	// used when Node.name is empty
	Name string

	ReadAll func(context.Context, *Node) ([]byte, error)
	Write   func(context.Context, *Node, []byte) error
}

type DirNode struct {
	Node
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
	target *Node
}

func (sn *SymlinkNode) Attr(a *fuse.Attr) {
	a.Inode = sn.ino
	a.Mode = os.ModeSymlink | sn.mode
}

type AttrNode struct {
	Node
	ty *AttrType
}

func (an *AttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.ino
	a.Mode = an.mode
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

func NewAttrNode(parent *DirNode, ty *AttrType, priv interface{}) (*AttrNode, error) {
	name := ty.Name
	an := new(AttrNode)
	err := an.Node.Init(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s', %#v): %s", name, priv, err)
	}
	an.ty = ty

	if ty.ReadAll != nil {
		an.mode |= 0444
	}
	if ty.Write != nil {
		an.mode |= 0222
	}

	return an, nil
}
