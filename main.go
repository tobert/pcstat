package main

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
 *
 * pcstat.go - page cache stat
 *
 * uses the mincore(2) syscall to find out which pages (almost always 4k)
 * of a file are currently cached in memory
 *
 */

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strings"

	pcstat "github.com/tobert/pcstat/pkg"
)

var (
	pidFlag                                     int
	terseFlag, nohdrFlag, jsonFlag, unicodeFlag bool
	plainFlag, ppsFlag, histoFlag, bnameFlag    bool
	sortFlag                                    bool
)

func init() {
	// TODO: error on useless/broken combinations
	flag.IntVar(&pidFlag, "pid", 0, "show all open maps for the given pid")
	flag.BoolVar(&terseFlag, "terse", false, "show terse output")
	flag.BoolVar(&nohdrFlag, "nohdr", false, "omit the header from terse & text output")
	flag.BoolVar(&jsonFlag, "json", false, "return data in JSON format")
	flag.BoolVar(&unicodeFlag, "unicode", false, "return data with unicode box characters")
	flag.BoolVar(&plainFlag, "plain", false, "return data with no box characters")
	flag.BoolVar(&ppsFlag, "pps", false, "include the per-page status in JSON output")
	flag.BoolVar(&histoFlag, "histo", false, "print a simple histogram instead of raw data")
	flag.BoolVar(&bnameFlag, "bname", false, "convert paths to basename to narrow the output")
	flag.BoolVar(&sortFlag, "sort", false, "sort output by cached pages desc")
}

func main() {
	flag.Parse()
	files := flag.Args()

	if pidFlag != 0 {
		maps := getPidMaps(pidFlag)
		files = append(files, maps...)
		pcstat.SwitchMountNs(pidFlag)
	}

	// all non-flag arguments are considered to be filenames
	// this works well with shell globbing
	// file order is preserved throughout this program
	if len(files) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	stats := make(PcStatusList, 0, len(files))
	for _, fname := range files {
		status, err := pcstat.GetPcStatus(fname)
		if err != nil {
			log.Printf("skipping %q: %v", fname, err)
			continue
		}

		// convert long paths to their basename with the -bname flag
		// this overwrites the original filename in pcs but it doesn't matter since
		// it's not used to access the file again -- and should not be!
		if bnameFlag {
			status.Name = path.Base(fname)
		}

		stats = append(stats, status)
	}

	if sortFlag {
		sort.Slice(stats, func(i, j int) bool {
			return stats[i].Cached > stats[j].Cached
		})
	}

	if jsonFlag {
		stats.formatJson(!ppsFlag)
	} else if terseFlag {
		stats.formatTerse()
	} else if histoFlag {
		stats.formatHistogram()
	} else if unicodeFlag {
		stats.formatUnicode()
	} else if plainFlag {
		stats.formatPlain()
	} else {
		stats.formatText()
	}
}

func getPidMaps(pid int) []string {
	fname := fmt.Sprintf("/proc/%d/maps", pid)

	f, err := os.Open(fname)
	if err != nil {
		log.Fatalf("could not open '%s' for read: %v", fname, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// use a map to help avoid duplicates
	maps := make(map[string]bool)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 6 && strings.HasPrefix(parts[5], "/") {
			// found something that looks like a file
			maps[parts[5]] = true
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("reading '%s' failed: %s", fname, err)
	}

	// convert back to a list
	out := make([]string, 0, len(maps))
	for key := range maps {
		out = append(out, key)
	}

	return out
}
