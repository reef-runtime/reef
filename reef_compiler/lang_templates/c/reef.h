#pragma once

#include "defs.h"
#include "log.h"
#include "walloc.h"

// Wasm Function Imports.
void reef_progress(float done) __attribute__((__import_module__("reef"), __import_name__("progress"), ));
void reef_sleep(float seconds) __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

// User main function definition
void run(uint8_t *dataset, size_t len);

// Result functions
void reef_result_int(int value);
void reef_result_bytes(uint8_t *ptr, size_t len);
void reef_result_string(char *ptr, size_t len);
