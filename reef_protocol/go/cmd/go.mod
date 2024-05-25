module github.com/reef-runtime/reef/reef_protocol/test

replace github.com/reef-runtime/reef/reef_protocol => ../

go 1.21.10

require (
	capnproto.org/go/capnp v2.18.2+incompatible
	github.com/reef-runtime/reef/reef_protocol v0.0.0-00010101000000-000000000000
)

require (
	capnproto.org/go/capnp/v3 v3.0.0-alpha-29 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9 // indirect
	zenhack.net/go/util v0.0.0-20230414204917-531d38494cf5 // indirect
	zombiezen.com/go/capnproto2 v2.18.2+incompatible // indirect
)
