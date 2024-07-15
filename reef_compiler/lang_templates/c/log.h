#pragma once
#pragma clang diagnostic ignored "-Wunknown-attributes"

#include "defs.h"

// Call to interpreter to log a string at the specified pointer with specified length
void reef_log(char *ptr, size_t num_bytes) __attribute__((__import_module__("reef"), __import_name__("log"), ));

// Logs a NULL-terminated string
void reef_puts(char *message);
// Logs an integer
void reef_log_int(int val);

// Calculates the length of NULL-terminated string
unsigned long strlen(const char *ptr);
// Converts an integer into a string
char *itoa(int value, char *result, int base);
