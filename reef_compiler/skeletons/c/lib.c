#include "./lib.h"
#include "walloc.c"

size_t reef_strlen(char *ptr) {
    int len = 0;
    while (ptr && ptr[len] != '\0') {
        len++;
    }

    return len;
}

#define BASE 10
void reef_log_int(int val) {
    // max len of 32bit int as dec in base 10
    char buf[10];

    int i;
    for (i = 0; i < BASE; i++) {
        buf[BASE - 1 - i] = '0' + (val % BASE);
        val = val / BASE;
        if (val == 0)
            break;
    }

    reef_log(buf + (BASE - 1 - i), i + 1);
}

// user main function declaration
#include "./input.c"

void reef_main() {
    size_t len = _reef_dataset_len();
    size_t pages = (len + PAGE_SIZE - 1) / PAGE_SIZE;

    // TODO: can we get this page aligned?
    uint8_t *dataset_mem = malloc(pages * PAGE_SIZE);

    _reef_dataset_write(dataset_mem);

    run(dataset_mem, len);
}
