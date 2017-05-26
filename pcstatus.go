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
	"errors"
	"fmt"
	"os"
	"time"
)

// page cache status
// Bytes: size of the file (from os.File.Stat())
// Pages: array of booleans: true if cached, false otherwise
type PcStatus struct {
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

func GetPcStatus(fname string) (PcStatus, error) {
	pcs := PcStatus{Name: fname}

	f, err := os.Open(fname)
	if err != nil {
		return pcs, fmt.Errorf("could not open file for read: %v", err)
	}
	defer f.Close()

	// TEST TODO: verify behavior when the file size is changing quickly
	// while this function is running. I assume that the size parameter to
	// mincore will prevent overruns of the output vector, but it's not clear
	// what will be in there when the file is truncated between here and the
	// mincore() call.
	fi, err := f.Stat()

	if err != nil {
		return pcs, fmt.Errorf("could not stat file: %v", err)
	}
	if fi.IsDir() {
		return pcs, errors.New("file is a directory")
	}

	pcs.Size = fi.Size()
	pcs.Timestamp = time.Now()
	pcs.Mtime = fi.ModTime()

	pcs.PPStat, err = FileMincore(f, fi.Size())

	if err != nil {
		return pcs, err
	}

	// count the number of cached pages
	for _, b := range pcs.PPStat {
		if b {
			pcs.Cached++
		}
	}
	pcs.Pages = len(pcs.PPStat)
	pcs.Uncached = pcs.Pages - pcs.Cached

	// convert to float for the occasional sparsely-cached file
	// see the README.md for how to produce one
	pcs.Percent = (float64(pcs.Cached) / float64(pcs.Pages)) * 100.00

	return pcs, nil
}
