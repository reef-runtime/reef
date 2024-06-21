#pragma once
#define PAGE_SIZE 65536

#include "attributes.h"

void *malloc(size_t size);

void free(void *ptr);
