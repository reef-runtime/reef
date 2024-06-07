INPUT_DIR:=schema
INPUT_FILES:=$(wildcard $(INPUT_DIR)/*.capnp)

.PHONY: test build init

init: 
	go install capnproto.org/go/capnp/v3/capnpc-go@latest

test: ./rust/src/*.rs go
	cd ./rust/ && cargo clippy

build: $(INPUT_FILES)
	echo "Compiling compiler.capnp to Go..."
	cd go && \
		PATH=$(PATH):~/go/bin \
		capnp compile -ogo ../$(INPUT_DIR)/compiler.capnp --src-prefix=../$(INPUT_DIR)/

