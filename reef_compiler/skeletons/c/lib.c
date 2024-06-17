#include "./lib.h"
#include "./input.c"

void reef_log(char *ptr, int bytes_len)
    __attribute__((__import_module__("reef"), __import_name__("log"), ));

int reef_strlen(char *ptr) {
  int len = 0;
  while (ptr && ptr[len] != '\0') {
    len++;
  }

  return len;
}

int reef_main(int arg) {
  char *msg = "Hello World!";
  int len = reef_strlen(msg);

  reef_log(msg, len);

#define DS_LEN 3
  byte dataset[DS_LEN] = {1, 2, 3};

  run(dataset, DS_LEN);

  return 42;
}
