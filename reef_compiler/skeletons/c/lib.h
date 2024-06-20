#pragma once
#pragma clang diagnostic ignored "-Wunknown-attributes"

#define byte unsigned char

void reef_log(char *ptr, int bytes_len)
    __attribute__((__import_module__("reef"), __import_name__("log"), ));

void reef_progress(float done)
    __attribute__((__import_module__("reef"), __import_name__("progress"), ));

void reef_sleep(float seconds)
    __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

int _reef_dataset_len(float seconds)
    __attribute__((__import_module__("reef"),
                   __import_name__("dataset_len"), ));

void _reef_dataset_write(int ptr)
    __attribute__((__import_module__("reef"),
                   __import_name__("dataset_write"), ));

int reef_strlen(char *ptr);
