package main

import (
	"os"

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
	root *Node
	// TODO(bp) locks
}

func (s *Super) Init() {
	s.seq.Init()
	// TODO(bp) init root node?
}

func (s *Super) NextInodeNum() uint64 {
	return s.seq.Next()
}

type Node struct {
	parent *Node
	name   string

	// usually a link back to the struct embedding this node
	priv interface{}

	dir     *Dir
	symlink *Symlink
	attr    *Attr

	ino  uint64
	mode os.FileMode
}

func (n *Node) Init(priv interface{}) {
	n.priv = priv
}

type Dir struct {
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
