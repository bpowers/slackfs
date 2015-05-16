// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
)

type DirOwner interface {
	DirNode() *DirNode
}

// DirCreator matches the signature of New{Channel,Group,IM}Dir
type DirCreator func(parent *DirNode, name string, priv interface{}) (*DirNode, error)

type DirSet struct {
	dn      *DirNode // us
	parent  *DirNode
	byName  *DirNode // by-name child dir
	byId    *DirNode // by-id child dir
	create  DirCreator
	objDirs map[string]*DirNode
	objSyms map[string]*SymlinkNode
}

func NewDirSet(parent *DirNode, name string, create DirCreator, priv interface{}) (ds *DirSet, err error) {
	ds = new(DirSet)
	ds.create = create
	ds.dn, err = NewDirNode(parent, name, priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode('%s'): %s", name, err)
	}
	ds.byName, err = NewDirNode(ds.dn, "by-name", priv)
	if err != nil {
		return nil, fmt.Errorf("NewDirNode(by-name): %s", err)
	}
	ds.byId, err = NewDirNode(ds.dn, "by-id", priv)
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

func (ds *DirSet) Add(id, name string, priv interface{}) error {
	child, err := (ds.create)(ds.byId, id, priv)
	if err != nil {
		return fmt.Errorf("ds.create(%s): %s", id, err)
	}

	ds.objDirs[id] = child
	s, err := NewSymlinkNode(ds.byName, name, child)
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
