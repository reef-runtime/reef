#pragma once

#include "defs.h"

#define PAGE_SIZE 65536

void *malloc(size_t size);
void free(void *ptr);
