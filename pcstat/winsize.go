package main

/*
 * Copyright 2015-2017 A. Tobey <tobert@gmail.com> @AlTobey
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * pcstat.go - page cache stat
 *
 * uses the mincore(2) syscall to find out which pages (almost always 4k)
 * of a file are currently cached in memory
 *
 */

import (
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// adapted from https://groups.google.com/d/msg/golang-nuts/8d4pOPmSL9Q/H6WUqbGNELEJ
type winsize struct {
	ws_row, ws_col       uint16
	ws_xpixel, ws_ypixel uint16
}

func getwinsize() winsize {
	ws := winsize{}
	_, _, err := unix.Syscall(syscall.SYS_IOCTL,
		uintptr(0), uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if err != 0 {
		log.Fatalf("TIOCGWINSZ failed to get terminal size: %s\n", err)
	}
	return ws
}
