## pcstat - get page cache statistics for files

A common question when tuning databases and other IO-intensive applications is,
"is Linux caching my data or not?" pcstat gets that information for you using
the mincore(2) syscall.

The fincore application from [linux-ftools](https://code.google.com/p/linux-ftools/) does the
same thing and I read its source code. I chose not to use it because it appears to be abandoned
and has some build problems.

## Usage

    pcstat filename
    pcstat -json filename > data.json

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

Al Tobey <tobert@gmail.com> @AlTobey

## License

Apache 2.0
