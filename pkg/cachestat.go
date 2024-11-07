package pcstat

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// FileCachestat uses the cachestat syscall to get the
// number of cached pages for the file without using '
// an intermediate per-page bool map. It then returns
// it alongside the numberof total pages
func FileCachestat(f *os.File, size int64) (*unix.Cachestat_t, int, error) {
	//skip could not mmap error when the file size is 0
	if int(size) == 0 {
		return nil, 0, nil
	}

	pcount := int((size + int64(os.Getpagesize()) - 1) / int64(os.Getpagesize()))

	// Use cachestat syscall
	crange := &unix.CachestatRange{
		Off: 0,
		Len: uint64(size),
	}
	cstat := &unix.Cachestat_t{}
	err := unix.Cachestat(uint(f.Fd()), crange, cstat, 0)
	if err != nil {
		return nil, 0, fmt.Errorf("cachestat syscall failed: %v", err)
	}

	return cstat, pcount, nil
}

