// Copyright 2016 Giulio Iotti. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package umarell

import (
	"log"
	"os"
)

type logger interface {
	Printf(string, ...interface{})
	Fatal(...interface{})
}

func newStdLogger() logger {
	return log.New(os.Stderr, "", log.LstdFlags)
}
