#include "reef.h"
#include "dataset.h"
#include "walloc.h"

//
// Main user code.
//

void reef_main() {
    size_t len = _reef_dataset_len();
    size_t pages = (len + PAGE_SIZE - 1) / PAGE_SIZE;

    // TODO: can we get this page aligned?
    uint8_t *dataset_mem = malloc(pages * PAGE_SIZE);

    _reef_dataset_write(dataset_mem);

    run(dataset_mem, len);
}
