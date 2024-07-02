#include "reef.h"

// Internal imports
#include "imports.h"

// #include "input.c"

//
// Wasm entry point
//
void reef_main() {
    size_t len = _reef_dataset_len();
    size_t len_alloc = (len + 7) & ~0x07;

    uint8_t *dataset_mem = malloc(len_alloc);
    _reef_dataset_write(dataset_mem);

    reef_result_int(0);

    run(dataset_mem, len);
}

// Result functions
void reef_result_int(int value) {
    uint8_t *ptr = (uint8_t *)&value;
    _reef_result(0, ptr, 4);
}
void reef_result_bytes(uint8_t *ptr, size_t len) { _reef_result(1, ptr, len); }
void reef_result_string(char *ptr, size_t len) { _reef_result(2, (uint8_t *)ptr, len); }
