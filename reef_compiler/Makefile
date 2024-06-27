CARGO:=cargo

RUN_PORT:=1234
BUILD_DIR:=./reef_build


.PHONY: run

run: ./src/*.rs
	$(CARGO) r $(RUN_PORT) $(BUILD_DIR) -c

test: ./src/*.rs
	$(CARGO) clippy
