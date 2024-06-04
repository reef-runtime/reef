CARGO:=cargo

RUN_PORT:=1234

.PHONY: run

run: ./src/*.rs
	$(CARGO) r server $(RUN_PORT)

test: ./src/*.rs
	$(CARGO) clippy
