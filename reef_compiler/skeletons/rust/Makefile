CARGO:=cargo
PACKAGE_NAME:=rust_test
TARGET_TRIPLE:=wasm32-unknown-unknown

COMPILATION_MODE:=release

IN_FILE:=input.rs
OUT_FILE:=output.wasm

target:
	$(CARGO) build --package $(PACKAGE_NAME) --target=$(TARGET_TRIPLE) --$(COMPILATION_MODE)
	cp ./target/$(TARGET_TRIPLE)/$(COMPILATION_MODE)/$(PACKAGE_NAME).wasm $(OUT_FILE)
	wasm-opt -o $(OUT_FILE) -O $(OUT_FILE)

build: target
