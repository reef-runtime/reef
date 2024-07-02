#include "../../../reef_compiler/lang_templates/c/reef.h"

#include "reef.h"

int fib(int n) {
  if (n < 2) {
    return n;
  }

  return fib(n - 2) + fib(n - 1);
}

void run(uint8_t *dataset, size_t ds_len) {
  reef_puts("Calculating Fibonacci Sequence");

  int I = 20;

  for (int x = 0; x < I; x++) {
    reef_sleep(1.0);
    float progress = (float)x / (float)I;
    reef_log_int(progress * 100);
    reef_progress(progress);
  }
}
