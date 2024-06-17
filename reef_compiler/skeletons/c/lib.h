void reef_log(char *ptr, int bytes_len)
    __attribute__((__import_module__("reef"), __import_name__("log"), ));

void reef_sleep()
    __attribute__((__import_module__("reef"), __import_name__("sleep"), ));

int reef_strlen(char *ptr);

//
// Reef usercode "main" function.
//

#define byte unsigned char

void run(byte *dataset, long dataset_size);
