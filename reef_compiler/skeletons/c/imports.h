#pragma once
#pragma clang diagnostic ignored "-Wunknown-attributes"

#include "defs.h"

//
// Wasm imports used only by Reef C std code, not user code
//

// dataset imports
size_t _reef_dataset_len() __attribute__((__import_module__("reef"), __import_name__("dataset_len"), ));
void _reef_dataset_write(uint8_t *ptr) __attribute__((__import_module__("reef"), __import_name__("dataset_write"), ));

// result import
void _reef_result(size_t result_type, uint8_t *ptr, size_t len)
    __attribute__((__import_module__("reef"), __import_name__("result"), ));
