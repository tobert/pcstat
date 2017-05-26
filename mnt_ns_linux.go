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
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// not available before Go 1.4
const CLONE_NEWNS = 0x00020000 /* mount namespace */

// if the pid is in a different mount namespace (e.g. Docker)
// the paths will be all wrong, so try to enter that namespace
func SwitchMountNs(pid int) {
	myns := getMountNs(os.Getpid())
	pidns := getMountNs(pid)

	if myns != pidns {
		setns(pidns)
	}
}

func getMountNs(pid int) int {
	fname := fmt.Sprintf("/proc/%d/ns/mnt")
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
	ret, _, err := unix.Syscall(SYS_SETNS, uintptr(uint(fd)), uintptr(CLONE_NEWNS), 0)
	if ret != 0 {
		return fmt.Errorf("syscall SYS_SETNS failed: %v", err)
	}

	return nil
}
