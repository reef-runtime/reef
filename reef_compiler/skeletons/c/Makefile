CLANG:=clang
CLANG_ARGS:=-Ofast
TARGET=wasm32

IN_FILES:=$(wildcard *.c)
IN_FILES := $(filter-out input.c, $(IN_FILES))
OUT_FILE:=./output.wasm

.PHONY: build

build:
	echo "$(IN_FILES)"

	$(CLANG) -Wl,--no-entry -Wl,--export-all \
		--target=$(TARGET) \
		--no-standard-libraries \
		$(CLANG_ARGS) \
		$(IN_FILES) \
		-o $(OUT_FILE)
	wasm-opt -o $(OUT_FILE) -O4 --strip-debug $(OUT_FILE)

