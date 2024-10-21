# srg-meshoptimizer-go

CGo bindings for the [meshoptimizer](https://github.com/zuex/meshoptimizer)
Vulkan library.


Built using [c-for-go](https://c.for-go.com).

## Installation

Go 1.23.2

`sudo apt install libmeshoptimizer-dev`

## Compilation

Make sure that your Go binaries are a part of your path variables.
Add the following to your bashrc.

```bash
export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
```

`go install github.com/xlab/c-for-go@latest`

`c-for-go meshoptimizer.yml`


## TODO

- [x] Generate Bindings
- [x] Fix `Allocate.PassRef()`
- [x] Test other versions

