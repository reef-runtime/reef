#include "../../../reef_compiler/skeletons/c/reef.h"

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

    int its = 29;
    for (int i = 0; i < its; i++) {
      char result[10] = {'0'};
      result[9] = 0;

      int res = fib(i);

      itoa(res, result, 10);

      reef_puts(result);
    }

    reef_progress((float)x / (float)I);
  }
}
