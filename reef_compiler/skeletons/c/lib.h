#pragma once
#pragma clang diagnostic ignored "-Wunknown-attributes"

typedef __SIZE_TYPE__ size_t;
typedef __UINTPTR_TYPE__ uintptr_t;
typedef __UINT8_TYPE__ uint8_t;

void reef_log(char *ptr, size_t bytes_len) __attribute__((__import_module__("reef"), __import_name__("log"), ));

void reef_progress(float done) __attribute__((__import_module__("reef"), __import_name__("progress"), ));

void reef_sleep(float seconds) __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

size_t _reef_dataset_len() __attribute__((__import_module__("reef"), __import_name__("dataset_len"), ));

void _reef_dataset_write(uint8_t *ptr) __attribute__((__import_module__("reef"), __import_name__("dataset_write"), ));

size_t reef_strlen(char *ptr);
void reef_log_int(int val);

// user main function definition
void run(uint8_t *dataset, size_t len);
