package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"unsafe"
)

// pcStat: page cache status
// Bytes: size of the file (from os.File.Stat())
// Pages: array of booleans: true if cached, false otherwise
type pcStat struct {
	Name     string `json:"filename"` // file name as specified on command line
	Size     int64  `json:"size"`     // file size in bytes
	Pages    int    `json:"pages"`    // total memory pages
	Cached   int    `json:"cached"`   // number of pages that are cached
	Uncached int    `json:"uncached"` // number of pages that are not cached
	PPStat   []bool `json:"status"`   // per-page status, true if cached, false otherwise
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
	// TODO: set a maximum padding length, possibly based on terminal info?
	maxName := 8
	for _, pcs := range stats {
		if len(pcs.Name) > maxName {
			maxName = len(pcs.Name)
		}
	}

	pad := strings.Repeat("-", maxName+2)
	hr := fmt.Sprintf("|%s+----------------+------------+-----------+---------|", pad)
	fmt.Println(hr)
	if !nohdrFlag {
		pad = strings.Repeat(" ", maxName-4)
		fmt.Printf("| Name%s | Size           | Pages      | Cached    | Percent |\n", pad)
		fmt.Println(hr)
	}

	for _, pcs := range stats {
		percent := int32((float64(pcs.Cached) / float64(pcs.Pages)) * 100)
		pad = strings.Repeat(" ", maxName-len(pcs.Name))
		fmt.Printf("| %s%s | %-15d| %-11d| %-10d| %-7d |\n", pcs.Name, pad, pcs.Size, pcs.Pages, pcs.Cached, percent)
	}

	fmt.Println(hr)
}

func (stats pcStatList) formatTerse() {
	if !nohdrFlag {
		fmt.Println("name,size,pages,cached,percent")
	}
	for _, pcs := range stats {
		percent := int32((float64(pcs.Cached) / float64(pcs.Pages)) * 100)
		fmt.Printf("%s,%d,%d,%d,%d\n", pcs.Name, pcs.Size, pcs.Pages, pcs.Cached, percent)
	}
}

func (stats pcStatList) formatJson() {
	// only show the per-page cache status if it's explicitly enabled
	// an empty "status": [] field will end up in the JSON but that's
	// not so bad since parsers will end up with support for both formats
	if !ppsFlag {
		for i, _ := range stats {
			stats[i].PPStat = []bool{}
		}
	}

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

	mmap, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_NONE, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("Could not mmap file '%s': %s\n", fname, err)
	}
	// TODO: check for MAP_FAILED which is ((void *) -1)
	// but maybe unnecessary since it looks like errno is always set when MAP_FAILED

	// one byte per page, only LSB is used, remainder is reserved and clear
	vecsz := (fi.Size() + int64(os.Getpagesize()) - 1) / int64(os.Getpagesize())
	vec := make([]byte, vecsz)

	mmap_ptr := uintptr(unsafe.Pointer(&mmap[0]))
	size_ptr := uintptr(fi.Size())
	vec_ptr := uintptr(unsafe.Pointer(&vec[0]))
	ret, _, err := syscall.RawSyscall(syscall.SYS_MINCORE, mmap_ptr, size_ptr, vec_ptr)
	if ret != 0 {
		log.Fatalf("syscall SYS_MINCORE failed: %s", err)
	}
	defer syscall.Munmap(mmap)

	pcs := pcStat{fname, fi.Size(), int(vecsz), 0, 0, make([]bool, vecsz)}

	if bnameFlag {
		pcs.Name = path.Base(fname)
	}

	// expose no bitshift only bool
	for i, b := range vec {
		if b%2 == 1 {
			pcs.PPStat[i] = true
			pcs.Cached++
		} else {
			pcs.PPStat[i] = false
			pcs.Uncached++
		}
	}

	return pcs
}
