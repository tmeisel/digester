package main

import (
	"flag"
	"fmt"

	"github.com/tmeisel/digester/pkg/digest"
)

const DefaultParallel = 10

func main() {
	parallel := parallel()
	urls := flag.Args()

	h := digest.New(parallel)

	for url, hash := range h.Run(urls...) {
		fmt.Printf("%s: %s\n", url, hash)
	}
}

func parallel() int {
	fParallel := flag.Int("parallel", DefaultParallel, "-parallel n")

	flag.Parse()

	if fParallel != nil && *fParallel > 0 {
		return *fParallel
	}

	return DefaultParallel
}
