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

	dir     *Dir
	symlink *Symlink
	attr    *Attr

	ino  uint64
	mode os.FileMode
}

func (n *Node) Init(parent *DirNode, name string, priv interface{}) error {
	if parent == nil {
		return fmt.Errorf("nil parent")
	}
	if name == "" {
		return fmt.Errorf("empty name")
	}
	n.super = parent.n.super
	n.parent = parent
	n.name = name

	n.priv = priv

	n.ino = parent.n.super.NextInodeNum()

	return nil
}

func (n *Node) DirentType() fuse.DirentType {
	switch {
	case n.dir != nil && n.symlink == nil && n.attr == nil:
		return fuse.DT_Dir
	case n.dir == nil && n.symlink != nil && n.attr == nil:
		return fuse.DT_Link
	case n.dir == nil && n.symlink == nil && n.attr != nil:
		return fuse.DT_File
	default:
		panic("inconsistent direnttype")
	}
}

func (n *Node) IsDir() bool {
	return n.dir != nil
}

// Activate exposes this Node in the filesystem
func (n *Node) Activate() error {
	if n.parent == nil {
		return nil
	}

	return n.parent.addChild(n)
}

type Dir struct {
	childmap map[string]*Node
	children []*Node
}

type Symlink struct {
	path   string
	target *Node
}

type AttrType struct {
	// used when Node.name is empty
	Name string

	ReadAll func(context.Context, *Node) ([]byte, error)
	Write   func(context.Context, *Node, []byte) error
}

type Attr struct {
	ty *AttrType
}

type DirNode struct {
	n Node
	d Dir
}

func (dn *DirNode) addChild(child *Node) error {
	dn.d.childmap[child.name] = child
	dn.d.children = append(dn.d.children, child)
	return nil
}

func (dn *DirNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if n, ok := dn.d.childmap[name]; ok {
		return n, nil
	} else {
		return nil, fuse.ENOENT
	}
}

func (dn *DirNode) Attr(a *fuse.Attr) {
	a.Inode = dn.n.ino
	a.Mode = dn.n.mode

	// linkcount for dirs is 2 + n subdirs
	a.Nlink = 2
	for _, n := range dn.d.children {
		if n.IsDir() {
			a.Nlink++
		}
	}
}

func (dn *DirNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dents := make([]fuse.Dirent, 0, len(dn.d.children))
	for _, n := range dn.d.children {
		dents = append(dents, fuse.Dirent{n.ino, n.DirentType(), n.name})
	}
	return dents, nil
}

type SymNode struct {
	n Node
	s Symlink
}

type AttrNode struct {
	n Node
	a Attr
}

func (an *AttrNode) Attr(a *fuse.Attr) {
	a.Inode = an.n.ino
	a.Mode = an.n.mode
}

func NewSuper() *Super {
	super := new(Super)
	super.Init()

	root := new(DirNode)

	// FIXME(bp) open coded from Node.Init
	root.n.super = super
	root.n.ino = super.NextInodeNum()

	// FIXME(bp) open coded from NewDirNode
	root.n.dir = &root.d
	root.d.childmap = make(map[string]*Node)
	root.d.children = make([]*Node, 0)
	root.n.mode = os.ModeDir | 0555

	super.root = root

	return super
}

func NewDirNode(parent *DirNode, name string, priv interface{}) (*DirNode, error) {
	dn := new(DirNode)
	err := dn.n.Init(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s', %#v): %s", name, priv, err)
	}
	dn.n.dir = &dn.d
	dn.d.childmap = make(map[string]*Node)
	dn.d.children = make([]*Node, 0)

	dn.n.mode = os.ModeDir | 0555

	return dn, nil
}

func NewAttrNode(parent *DirNode, ty *AttrType, priv interface{}) (*AttrNode, error) {
	name := ty.Name
	an := new(AttrNode)
	err := an.n.Init(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("n.Init('%s', %#v): %s", name, priv, err)
	}
	an.n.attr = &an.a
	an.a.ty = ty

	if ty.ReadAll != nil {
		an.n.mode |= 0444
	}
	if ty.Write != nil {
		an.n.mode |= 0222
	}

	return an, nil
}
