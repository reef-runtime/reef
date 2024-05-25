package main

import (
	"fmt"

	"capnproto.org/go/capnp/v3"
	reef "github.com/reef-runtime/reef/reef_protocol"
)

func main() {
	arena := capnp.SingleSegment(nil)

	_, seg, err := capnp.NewMessage(arena)
	if err != nil {
		panic(err.Error())
	}

	log, err := reef.NewJobLogMessage(seg)
	if err != nil {
		panic(err.Error())
	}

	fmt.Printf("Main: %v\n", log)
}
