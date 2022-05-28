# mört :fish:

[![Build Status](https://travis-ci.com/tomyl/mort.svg?branch=master)](https://travis-ci.org/tomyl/mort)
[![Go Report Card](https://goreportcard.com/badge/github.com/tomyl/mort)](https://goreportcard.com/report/github.com/tomyl/mort)

A simple console-based task management tool.

**Pre-alpha software**. Expect plenty of bugs and data loss.

# Build error?

```
$ go build
# github.com/mattn/go-sqlite3
sqlite3-binding.c: In function ‘sqlite3SelectNew’:
sqlite3-binding.c:125801:10: warning: function may return address of local variable [-Wreturn-local-addr]
125801 |   return pNew;
       |          ^~~~
sqlite3-binding.c:125761:10: note: declared here
125761 |   Select standin;
       |          ^~~~~~~
$ export CGO_CFLAGS="-g -O2 -Wno-return-local-addr"
$ go build
```

# TODO
- [ ] Add documentation and screenshots.
- [ ] Finish this TODO list.
