#include "defs.h"

void *memcpy(void *destination, void *source, size_t num) {
  char *csrc = (char *)source;
  char *cdest = (char *)destination;

  for (int i=0; i<num; i++) {
    cdest[i] = csrc[i];
  }

  return destination;
}
