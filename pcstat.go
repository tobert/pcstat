package main

/*
 * Copyright 2014 Albert P. Tobey <atobey@datastax.com> @AlTobey
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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// pcStat: page cache status
// Bytes: size of the file (from os.File.Stat())
// Pages: array of booleans: true if cached, false otherwise
type pcStat struct {
	Name      string    `json:"filename"`  // file name as specified on command line
	Size      int64     `json:"size"`      // file size in bytes
	Timestamp time.Time `json:"timestamp"` // time right before calling mincore
	Mtime     time.Time `json:"mtime"`     // last modification time of the file
	Pages     int       `json:"pages"`     // total memory pages
	Cached    int       `json:"cached"`    // number of pages that are cached
	Uncached  int       `json:"uncached"`  // number of pages that are not cached
	Percent   float64   `json:"percent"`   // percentage of pages cached
	PPStat    []bool    `json:"status"`    // per-page status, true if cached, false otherwise
}

type pcStatList []pcStat

var (
	terseFlag, nohdrFlag, jsonFlag, ppsFlag, bnameFlag bool
)

func init() {
	// TODO: error on useless/broken combinations
	flag.BoolVar(&terseFlag, "terse", false, "show terse output")
	flag.BoolVar(&nohdrFlag, "nohdr", false, "omit the header from terse & text output")
	flag.BoolVar(&jsonFlag, "json", false, "return data in JSON format")
	flag.BoolVar(&ppsFlag, "pps", false, "include the per-page status in JSON output")
	flag.BoolVar(&bnameFlag, "bname", false, "convert paths to basename to narrow the output")
}

func main() {
	flag.Parse()

	// all non-flag arguments are considered to be filenames
	// this works well with shell globbing
	// file order is preserved throughout this program
	stats := make(pcStatList, len(flag.Args()))

	for i, fname := range flag.Args() {
		stats[i] = getMincore(fname)
	}

	if jsonFlag {
		stats.formatJson()
	} else if terseFlag {
		stats.formatTerse()
	} else {
		stats.formatText()
	}
}

func (stats pcStatList) formatText() {
	// find the longest filename in the data for calculating whitespace padding
	maxName := 8
	for _, pcs := range stats {
		if len(pcs.Name) > maxName {
			maxName = len(pcs.Name)
		}
	}

	// create horizontal grid line
	pad := strings.Repeat("-", maxName+2)
	hr := fmt.Sprintf("|%s+----------------+------------+-----------+---------|", pad)

	fmt.Println(hr)

	// -nohdr may be chosen to save 2 lines of precious vertical space
	if !nohdrFlag {
		pad = strings.Repeat(" ", maxName-4)
		fmt.Printf("| Name%s | Size           | Pages      | Cached    | Percent |\n", pad)
		fmt.Println(hr)
	}

	for _, pcs := range stats {
		pad = strings.Repeat(" ", maxName-len(pcs.Name))

		// %07.3f was chosen to make it easy to scan the percentages vertically
		// I tried a few different formats only this one kept the decimals aligned
		fmt.Printf("| %s%s | %-15d| %-11d| %-10d| %07.3f |\n",
			pcs.Name, pad, pcs.Size, pcs.Pages, pcs.Cached, pcs.Percent)
	}

	fmt.Println(hr)
}

func (stats pcStatList) formatTerse() {
	if !nohdrFlag {
		fmt.Println("name,size,timestamp,mtime,pages,cached,percent")
	}
	for _, pcs := range stats {
		time := pcs.Timestamp.Unix()
		mtime := pcs.Mtime.Unix()
		fmt.Printf("%s,%d,%d,%d,%d,%d,%g\n",
			pcs.Name, pcs.Size, time, mtime, pcs.Pages, pcs.Cached, pcs.Percent)
	}
}

func (stats pcStatList) formatJson() {
	b, err := json.Marshal(stats)
	if err != nil {
		log.Fatalf("JSON formatting failed: %s\n", err)
	}
	os.Stdout.Write(b)
	fmt.Println("")
}

func getMincore(fname string) pcStat {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("Could not open file '%s' for read: %s\n", fname, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Fatalf("Could not stat file %s: %s\n", fname, err)
	}
	if fi.Size() == 0 {
		log.Fatalf("%s appears to be 0 bytes in length\n", fname)
	}

	// []byte slice
	mmap, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_NONE, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("Could not mmap file '%s': %s\n", fname, err)
	}
	// TODO: check for MAP_FAILED which is ((void *) -1)
	// but maybe unnecessary since it looks like errno is always set when MAP_FAILED

	// one byte per page, only LSB is used, remainder is reserved and clear
	vecsz := (fi.Size() + int64(os.Getpagesize()) - 1) / int64(os.Getpagesize())
	vec := make([]byte, vecsz)

	// get all of the arguments to the mincore syscall converted to uintptr
	mmap_ptr := uintptr(unsafe.Pointer(&mmap[0]))
	size_ptr := uintptr(fi.Size())
	vec_ptr := uintptr(unsafe.Pointer(&vec[0]))

	// get the timestamp right before the syscall
	ts := time.Now()

	// use Go's ASM to submit directly to the kernel, no C wrapper needed
	// mincore(2): int mincore(void *addr, size_t length, unsigned char *vec);
	// 0 on success, takes the pointer to the mmap, a size, which is the
	// size that came from f.Stat(), and the vector, which is a pointer
	// to the memory behind an []byte
	// this writes a snapshot of the data into vec which a list of 8-bit flags
	// with the LSB set if the page in that position is currently in VFS cache
	ret, _, err := syscall.RawSyscall(syscall.SYS_MINCORE, mmap_ptr, size_ptr, vec_ptr)
	if ret != 0 {
		log.Fatalf("syscall SYS_MINCORE failed: %s", err)
	}
	defer syscall.Munmap(mmap)

	pcs := pcStat{fname, fi.Size(), ts, fi.ModTime(), int(vecsz), 0, 0, 0.0, []bool{}}

	// only export the per-page cache mapping if it's explicitly enabled
	// an empty "status": [] field, but NBD.
	if ppsFlag {
		pcs.PPStat = make([]bool, vecsz)

		// there is no bitshift only bool
		for i, b := range vec {
			if b%2 == 1 {
				pcs.PPStat[i] = true
			} else {
				pcs.PPStat[i] = false
			}
		}

	}

	// convert long paths to their basename with the -bname flag
	// this overwrites the original filename in pcs but it doesn't matter since
	// it's not used to access the file again -- and should not be!
	if bnameFlag {
		pcs.Name = path.Base(fname)
	}

	for _, b := range vec {
		if b%2 == 1 {
			pcs.Cached++
		} else {
			pcs.Uncached++
		}
	}

	// convert to float for the occasional sparsely-cached file
	// see the README.md for how to produce one
	pcs.Percent = (float64(pcs.Cached) / float64(pcs.Pages)) * 100.00

	return pcs
}
