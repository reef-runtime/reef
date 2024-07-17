#include "defs.h"

void *memcpy(void *destination, void *source, size_t num) {
    char *csrc = (char *)source;
    char *cdest = (char *)destination;

    for (int i = 0; i < num; i++) {
        cdest[i] = csrc[i];
    }

    return destination;
}

void *memset(void *dest, int value, size_t num) {
    unsigned char *dst = dest;
    while (num > 0) {
        *dst = (unsigned char)value;
        dst++;
        num--;
    }
    return dest;
}
