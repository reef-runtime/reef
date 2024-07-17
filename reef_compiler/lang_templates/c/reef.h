#pragma once

#include "defs.h"
#include "log.h"
#include "walloc.h"
#include "memory.h"

// Wasm Function Imports.
void reef_progress(float done) __attribute__((__import_module__("reef"), __import_name__("progress"), ));
void reef_sleep(float seconds) __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

// User main function definition
void run(uint8_t *dataset, size_t len);

// Result functions
void reef_result_int(int value);
void reef_result_bytes(uint8_t *ptr, size_t len);
void reef_result_string(char *ptr, size_t len);

// Conversion functions
uint32_t *from_little_endian(uint8_t *arr, size_t n_bytes);
uint8_t *to_little_endian(uint32_t *arr, size_t n_bytes);
uint32_t *from_big_endian(uint8_t *arr, size_t n_bytes);
uint8_t *to_big_endian(uint32_t *arr, size_t n_bytes);
