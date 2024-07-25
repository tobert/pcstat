package pcstat

/*
 * Copyright 2014-2017 A. Tobey <tobert@gmail.com> @AlTobey
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
 */

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// if the pid is in a different mount namespace (e.g. Docker)
// the paths will be all wrong, so try to enter that namespace
func SwitchMountNs(pid int) {
	myns := getMountNs(os.Getpid())
	pidns := getMountNs(pid)

	if myns != pidns {
		setns(pid)
	}
}

func getMountNs(pid int) int {
	fname := fmt.Sprintf("/proc/%d/ns/mnt", pid)
	nss, err := os.Readlink(fname)

	// probably permission denied or namespaces not compiled into the kernel
	// ignore any errors so ns support doesn't break normal usage
	if err != nil || nss == "" {
		return 0
	}

	nss = strings.TrimPrefix(nss, "mnt:[")
	nss = strings.TrimSuffix(nss, "]")
	ns, err := strconv.Atoi(nss)

	// not a number? weird ...
	if err != nil {
		log.Fatalf("strconv.Atoi('%s') failed: %s\n", nss, err)
	}

	return ns
}

func setns(fd int) error {
	// Lock the system thread to prevent the goroutine from
	// switching to another system thread after call setnx
	runtime.LockOSThread()

	// Go runtime call clone to create a thread, and the flags parameter passed contains CLONE_FS.
	// Only by calling unshare to remove CLONE_FS then can setns set CLONE_NEWNS ok.
	// See man 2 setnx get more information.
	if err := unix.Unshare(unix.CLONE_FS); err != nil {
		return fmt.Errorf("unshare mount namespace error: %w", err)
	}

	nsMountFileName := fmt.Sprintf("/proc/%d/ns/mnt", fd)
	nsMountFile, err := os.Open(nsMountFileName)
	if err != nil {
		return fmt.Errorf("open mount namespace file error: %w", err)
	}
	defer nsMountFile.Close()

	if err = unix.Setns(int(nsMountFile.Fd()), unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("setns mount namespace error: %w", err)
	}
	return nil
}
