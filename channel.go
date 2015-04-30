// Copyright 2015 Bobby Powers. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/nlopes/slack"
)

type Channel struct {
	slack.Channel
	fs *FSConn
}
