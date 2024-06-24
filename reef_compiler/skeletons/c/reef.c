#include "reef.h"

// Internal imports
#include "imports.h"

// #include "input.c"

//
// Wasm entry point
//
void reef_main() {
    size_t len = _reef_dataset_len();
    size_t pages = (len + PAGE_SIZE - 1) / PAGE_SIZE;

    // TODO: can we get this page aligned?
    uint8_t *dataset_mem = malloc(pages * PAGE_SIZE);

    _reef_dataset_write(dataset_mem);

    run(dataset_mem, len);

    char result[3] = {1, 2, 3};
    _reef_result(1, (uint8_t *)result, 3);
}
