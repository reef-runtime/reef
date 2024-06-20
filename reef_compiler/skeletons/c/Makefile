CLANG:=clang
CLANG_ARGS:=
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
