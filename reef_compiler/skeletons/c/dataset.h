#pragma once
#include "attributes.h"

size_t _reef_dataset_len() __attribute__((__import_module__("reef"), __import_name__("dataset_len"), ));

void _reef_dataset_write(uint8_t *ptr) __attribute__((__import_module__("reef"), __import_name__("dataset_write"), ));
