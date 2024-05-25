void reef_log(char *ptr, int bytes_len)
    __attribute__((__import_module__("reef"), __import_name__("reef_log"),
                   __warn__unused__result__));

// void print_dummy(char *str_ptr, int length) {}

int reef_strlen(char *ptr) {
  int len = 0;
  while (ptr && ptr[len] != '\0') {
    len++;
  }

  return len;
}

int main() {
  char *msg = "Hello World!";
  int len = reef_strlen(msg);

  reef_log(msg, len);
  // print_dummy(msg, len);
}

void _start() { main(); }
