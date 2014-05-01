package main

import (
	"flag"
	"log"
	"os"
	"syscall"
	"unsafe"
)

// pcStat: page cache status
// Bytes: size of the file (from os.File.Stat())
// Pages: array of booleans: true if cached, false otherwise
type pcStat struct {
	Name     string
	Bytes    int64
	Pages    int
	Cached   int
	Uncached int
	Status   []bool
}

func main() {
	flag.Parse()
	for _, fname := range flag.Args() {
		pcs := getMincore(fname)
		percent := (pcs.Cached / pcs.Pages) * 100
		log.Printf("%s: Size: %d bytes, Pages Cached %d, Uncached: %d, %d%% cached\n",
			pcs.Name, pcs.Bytes, pcs.Cached, pcs.Uncached, percent)
	}
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

	// expose no bitshift only bool
	for i, b := range vec {
		if b%2 == 1 {
			pcs.Status[i] = true
			pcs.Cached++
		} else {
			pcs.Status[i] = false
			pcs.Uncached++
		}
	}

	return pcs
}
