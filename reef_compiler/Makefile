CARGO:=cargo

RUN_PORT:=1234
BUILD_DIR:=./reef_build
SKELETONS_DIR:=./skeletons/


.PHONY: run

run: ./src/*.rs
	$(CARGO) r server $(RUN_PORT) $(SKELETONS_DIR) $(BUILD_DIR) -c

test: ./src/*.rs
	$(CARGO) clippy
