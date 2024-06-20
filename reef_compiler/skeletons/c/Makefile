CLANG:=clang
CLANG_ARGS:=-Ofast
TARGET=wasm32

ENTRY_FILE:=lib.c

IN_FILE:=./input.c
OUT_FILE:=./output.wasm

.PHONY: build

build:
	$(CLANG) -Wl,--no-entry -Wl,--export-all \
		--target=$(TARGET) \
		--no-standard-libraries \
		$(CLANG_ARGS) \
		-o $(OUT_FILE) \
		$(ENTRY_FILE)
	wasm-opt -o $(OUT_FILE) -O4 --strip-debug $(OUT_FILE)

