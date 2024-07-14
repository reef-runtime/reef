#include "log.h"

size_t strlen(char *ptr) {
    int len = 0;
    while (ptr && ptr[len] != '\0') {
        len++;
    }

    return len;
}

void reef_puts(char *message) {
    size_t len = strlen(message);
    reef_log(message, len);
}

void reef_log_int(int val) {
    // max len of 32bit int as dec in base 10 + NULL
    char buf[11];

    itoa(val, buf, 10);

    reef_log(buf, strlen(buf));
}

// Taken from: http://www.strudel.org.uk/itoa/
char *itoa(int value, char *result, int base) {
    // check that the base if valid
    if (base < 2 || base > 36) {
        *result = '\0';
        return result;
    }

    char *ptr = result, *ptr1 = result, tmp_char;
    int tmp_value;

    do {
        tmp_value = value;
        value /= base;
        *ptr++ =
            "zyxwvutsrqponmlkjihgfedcba9876543210123456789abcdefghijklmnopqrstuvwxyz"[35 + (tmp_value - value * base)];
    } while (value);

    // Apply negative sign.
    if (tmp_value < 0)
        *ptr++ = '-';
    *ptr-- = '\0';

    // Reverse the string.
    while (ptr1 < ptr) {
        tmp_char = *ptr;
        *ptr-- = *ptr1;
        *ptr1++ = tmp_char;
    }
    return result;
}
