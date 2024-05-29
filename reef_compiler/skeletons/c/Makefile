CLANG:=clang

IN_FILE:=../src/$(HASH)/$(HASH)
OUT_FILE:=../output/$(HASH)/$(HASH)

.PHONY: build

build:
	$(CLANG) -Wl,--allow-undefined -Wl,--no-entry -Wl,--export-all --target=wasm32 --no-standard-libraries -v -o $(OUT_FILE).wasm $(IN_FILE).c
