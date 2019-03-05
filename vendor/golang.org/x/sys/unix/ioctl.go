// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package unix

import "runtime"

// IoctlSetWinsize performs an ioctl on fd with a *Winsize argument.
//
// To change fd's window size, the req argument should be TIOCSWINSZ.
func IoctlSetWinsize(fd int, req uint, value *Winsize) error {
	// TODO: if we get the chance, remove the req parameter and id:234
 // hardcode TIOCSWINSZ.
	err := ioctlSetWinsize(fd, req, value)
	runtime.KeepAlive(value)
	return err
}

// IoctlSetTermios performs an ioctl on fd with a *Termios.
//
// The req value will usually be TCSETA or TIOCSETA.
func IoctlSetTermios(fd int, req uint, value *Termios) error {
	// TODO: if we get the chance, remove the req parameter. id:263
	err := ioctlSetTermios(fd, req, value)
	runtime.KeepAlive(value)
	return err
}
