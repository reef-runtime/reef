#!make
SHELL:=/usr/bin/env bash

ENV_FILE=../.env
include $(ENV_FILE)
export $(shell sed 's/=.*//' $(ENV_FILE))

CARGO:=cargo

BUILD_DIR:=./reef_build


.PHONY: run

run: ./src/*.rs
	$(CARGO) r -- -p $(REEF_COMPILER_PORT) -b $(BUILD_DIR) -c

test: ./src/*.rs
	$(CARGO) clippy
