## pcstat - get page cache statistics for files

A common question when tuning databases and other IO-intensive applications is,
"is Linux caching my data or not?" pcstat gets that information for you using
the mincore(2) syscall.

The fincore application from [linux-ftools](https://code.google.com/p/linux-ftools/) does the
same thing and I read its source code. I chose not to use it because it appears to be abandoned
and has some build problems.

I wrote this is so that Apache Cassandra users can see if ssTables are being
cached. If $GOPATH/bin is in your PATH, this will get it installed:

    go get github.com/tobert/pcstat
    pcstat /var/lib/cassandra/data/*/*/*-Data.db

Alternatively, throw a binary on a webserver and pull it down with wget.
The URL below is real but not guaranteed to work.

    wget http://tobert.org/downloads/pcstat
    chmod 755 pcstat
    ./pcstat /var/lib/cassandra/data/*/*/*-Data.db

## Usage

Command-line arguments are described below. Every argument following the program
flags is considered a file for inspection.

```
pcstat <-json <-pps>|-terse|-default> <-nohdr> <-bname> file file file
 -json output will be JSON
   -pps include the per-page information in the output (can be huge!)
 -terse print terse machine-parseable output
 -default print ascii tables
 -nohdr don't print the column header in terse or default format
 -bname use basename(file) in the output (use for long paths)

Examples:
 pcstat filename
 pcstat -json filename > data.json

```

## Testing

The easiest way to tell if this tool is working is to drop caches and do reads on files to
get things into cache.

```
atobey@brak ~/src/pcstat $ dd if=/dev/urandom of=testfile bs=1M count=10
10+0 records in
10+0 records out
10485760 bytes (10 MB) copied, 0.805698 s, 13.0 MB/s
atobey@brak ~/src/pcstat $ ./pcstat testfile
|--------------------+----------------+------------+-----------+---------|
| Name               | Size           | Pages      | Cached    | Percent |
|--------------------+----------------+------------+-----------+---------|
| testfile           | 10485760       | 2560       | 2560      | 100     |
|--------------------+----------------+------------+-----------+---------|
atobey@brak ~/src/pcstat $ echo 1 |sudo tee /proc/sys/vm/drop_caches
1
atobey@brak ~/src/pcstat $ ./pcstat testfile
|--------------------+----------------+------------+-----------+---------|
| Name               | Size           | Pages      | Cached    | Percent |
|--------------------+----------------+------------+-----------+---------|
| testfile           | 10485760       | 2560       | 0         | 0       |
|--------------------+----------------+------------+-----------+---------|
atobey@brak ~/src/pcstat $ dd if=/dev/urandom of=testfile bs=4096 seek=10 count=1 conv=notrunc
1+0 records in
1+0 records out
4096 bytes (4.1 kB) copied, 0.000468208 s, 8.7 MB/s
atobey@brak ~/src/pcstat $ ./pcstat testfile
|--------------------+----------------+------------+-----------+---------|
| Name               | Size           | Pages      | Cached    | Percent |
|--------------------+----------------+------------+-----------+---------|
| testfile           | 10485760       | 2560       | 1         | 0       |
|--------------------+----------------+------------+-----------+---------|
```

## Building

    git clone https://github.com/tobert/pcstat.git
    cd pcstat
    go build
    sudo cp -a pcstat /usr/local/bin
    pcstat /usr/local/bin/pcstat

## Requirements

Any Go 1.x or higher should work fine. No external libraries are required.

From the mincore(2) man page:

* Available since Linux 2.3.99pre1 and glibc 2.2.
* mincore() is not specified in POSIX.1-2001, and it is not available on all UNIX implementations.
* Before kernel 2.6.21, mincore() did not return correct information some mappings.

## Author

Al Tobey <tobert@datastax.com> @AlTobey

## License

Apache 2.0
