#include "log.h"

size_t reef_strlen(char *ptr) {
    int len = 0;
    while (ptr && ptr[len] != '\0') {
        len++;
    }

    return len;
}

void reef_puts(char *message) {
    size_t len = reef_strlen(message);
    reef_log(message, len);
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
