[import.name/make](https://pkg.go.dev/import.name/make) is a build system where
you specify tasks by writing Go code.  It is not intended for building Go code,
as the Go toolchain already has that covered.  Instead, it can be used to build
non-Go artifacts of projects that already depend on the Go toolchain.

Add file `make.go` to your project root:

```go
//go:build ignore

package main

import (
	. "import.name/make"
)

func main() {
	Main(targets,
		"make.go", // These files are universal dependencies: if they
		"go.mod",  // are modified, all targets need to be rebuilt.
	)
}

func targets() (targets Tasks) {
	var (
		CC = Getvar("CC", "gcc")
	)

	sources := Globber("src/*.c")

	myTarget := targets.Add(Target("mytarget",
		If(Outdated("mytarget", sources),
			Command(CC, "-c", "-o", "example.o", sources),
			Command(CC, "-o", "mytarget", "example.o"),
		),
	))

	// ...

	return
}
```

Build your project by invoking:

	go run make.go
	go run make.go mytarget
	go run make.go mytarget another-target FOO=bar BAZ=quux

Show usage and list available targets and variables:

	go run make.go -h

See a [practical example](https://github.com/gate-computer/gate/blob/main/make.go).

