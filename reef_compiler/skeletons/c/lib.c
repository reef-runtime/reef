#include "./lib.h"

int reef_strlen(char *ptr) {
  int len = 0;
  while (ptr && ptr[len] != '\0') {
    len++;
  }

  return len;
}

// user main function definition
void run(byte *dataset, int len);
// user main function declaration
#include "./input.c"

void reef_main() {
#define DS_LEN 3
  byte dataset[DS_LEN] = {1, 2, 3};

  run(dataset, DS_LEN);
}
