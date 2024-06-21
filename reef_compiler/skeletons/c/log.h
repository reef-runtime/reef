#pragma once
#include "attributes.h"

void reef_log(char *ptr, size_t num_bytes) __attribute__((__import_module__("reef"), __import_name__("log"), ));

size_t reef_strlen(char *ptr);

void reef_log_int(int val);

// String behind `message` must be NUL-terminated.
void reef_puts(char *message);
