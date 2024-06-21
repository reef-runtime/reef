#pragma once

#include "attributes.h"
#include "dataset.h"
#include "fmt.h"
#include "log.h"
#include "walloc.h"

//
// Functions.
//

void reef_progress(float done) __attribute__((__import_module__("reef"), __import_name__("progress"), ));

void reef_sleep(float seconds) __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

// user main function definition
void run(uint8_t *dataset, size_t len);
