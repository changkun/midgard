# mainthread [![PkgGoDev](https://pkg.go.dev/badge/golang.design/x/mainthread)](https://pkg.go.dev/golang.design/x/mainthread) ![mainthread](https://github.com/golang-design/mainthread/workflows/mainthread/badge.svg?branch=main) ![](https://changkun.de/urlstat?mode=github&repo=golang-design/mainthread)

schedule functions to run on the main thread

```go
import "golang.design/x/mainthread"
```

## Features

- Main thread scheduling
- Schedule functions without memory allocation

## API Usage

Package mainthread offers facilities to schedule functions
on the main thread. To use this package properly, one must
call `mainthread.Init` from the main package. For example:

```go
package main

import "golang.design/x/mainthread"

func main() { mainthread.Init(fn) }

// fn is the actual main function 
func fn() {
	// ... do whatever you want to do ...

	// mainthread.Call returns when f1 returns. Note that if f1 blocks
	// it will also block the execution of any subsequent calls on the
	// main thread.
	mainthread.Call(f1)

	// ... do whatever you want to do ...

	// mainthread.Go returns immediately and f2 is scheduled to be
	// executed in the future.
	mainthread.Go(f2)

	// ... do whatever you want to do ...
}

func f1() { ... }
func f2() { ... }
```

## When do you need this package?

Read this to learn more about the design purpose of this package:
https://golang.design/research/zero-alloc-call-sched/

## Who is using this package?

The initial purpose of building this package is to support writing
graphical applications in Go. To know projects that are using this
package, check our [wiki](https://github.com/golang-design/mainthread/wiki)
page.


## License

MIT | &copy; 2021 The golang.design Initiative Authors, written by [Changkun Ou](https://changkun.de).