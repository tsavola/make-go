[import.name/make](https://pkg.go.dev/import.name/make) is a build system where
you specify tasks by writing Go code.  It is not intended for building Go code,
as the Go toolchain already has that covered.  Instead, it can be used to build
non-Go artifacts of projects that already depend on the Go toolchain.

Add file `make.go` to your project root:

```go
//go:build ignore
// +build ignore

package main

import . "import.name/make"

func main() { Main(targets, "make.go", "go.mod") }

func targets() (targets Tasks) {
	// ...
	return
}
```

Build your project by invoking:

	go run make.go
	go run make.go my-target
	go run make.go my-target another-target FOO=bar BAZ=quux

Show usage and list available targets and variables:

	go run make.go -h

See a [practical example](https://github.com/gate-computer/gate/blob/master/make.go).
If, after eyeballing that for a while, you wonder whether some feature is
supported, the answer is probably no (unless you're willing to implement it
yourself in Go).

