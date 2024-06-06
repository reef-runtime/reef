COMPILER:=capnp

INPUT_DIR:=./schema
INPUT_FILES:=$(wildcard $(INPUT_DIR)/*.capnp)

OUTPUT_FILES_GO:=$(patsubst $(INPUT_DIR)/%.capnp,%.go,$(INPUT_FILES))
GO_CAPNP_CHECKOUT:=./go-capnp

SCHEMA_PREFIX:=./schema/

.PHONY: test build init clean

clean:
	rm -rf $(GO_CAPNP_CHECKOUT)

init: $(GO_CAPNP_CHECKOUT)
	go install capnproto.org/go/capnp/v3/capnpc-go@latest

$(GO_CAPNP_CHECKOUT):
	git clone https://github.com/capnproto/go-capnp

go: $(OUTPUT_FILES_GO)

build: $(GO_CAPNP_CHECKOUT) go

test: ./rust/src/*.rs go
	cd ./rust/ && cargo clippy

$(OUTPUT_FILES_GO): $(INPUT_FILES)
	echo "Compiling " $< " to Go..."
	cd go && \
		PATH=$(PATH):~/go/bin \
		$(COMPILER) compile -I ../$(GO_CAPNP_CHECKOUT)/std -ogo ../$< --src-prefix=../$(SCHEMA_PREFIX)
