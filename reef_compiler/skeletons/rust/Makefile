SED:=sed
SED_PACKAGE_PLACEHOLER:=PLACEHOLDER

CARGO:=cargo
TARGET_TRIPLE:=wasm32-unknown-unknown

COMPILATION_MODE:=release

IN_FILE:=input.rs
OUT_FILE:=output.wasm

#
# Provided externally
#

HASH:=PLACEHOLDER
CRATE:=reef_$(HASH)

prepare:
	$(SED) -i "s/$(SED_PACKAGE_PLACEHOLER)/$(CRATE)/g" Cargo.toml

target:
	$(CARGO) build --target=$(TARGET_TRIPLE) --$(COMPILATION_MODE)
	cp ./target/$(TARGET_TRIPLE)/$(COMPILATION_MODE)/$(CRATE).wasm $(OUT_FILE)
	wasm-opt -o $(OUT_FILE) -O $(OUT_FILE)

build: prepare target
