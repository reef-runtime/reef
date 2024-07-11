#include "reef.h"

void greet_n_times(int n) {
  for (int i = 0; i < n; i++) {
    reef_puts("Hello World!");
    reef_sleep(0.2);
    reef_progress((float) i / (float) n);
  }
}

void run(uint8_t *dataset, size_t ds_len) {
  // Write your main code here, as if it
  // were the `main` function.
  greet_n_times(10);
}
