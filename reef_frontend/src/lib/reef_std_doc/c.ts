import { DocLang, DocSection } from '.';

const mainSection: DocSection = {
  name: 'User Main Function',
  entries: [
    {
      signature: 'void run(uint8_t *dataset, size_t dataset_len);',
      description: [
        'The entry function for a job written by Reef users. This function must be provided in each job submission because it will be called by wrapper code during execution on a Reef Node.',
        'As arguments you are passed a pointer to the specified dataset and the length of the dataset.',
      ],
    },
  ],
};

const reefSection: DocSection = {
  name: 'Reef Functions',
  description: ['Functions for interacting with the Reef system from jobs.'],
  entries: [
    {
      signature: 'void reef_progress(float done);',
      description: [
        'Reports the current process to the system.',
        'As an argument you have to pass the progress as a float value between 0 and 1.',
      ],
    },
    {
      signature: 'void reef_sleep(float seconds);',
      description: [
        'Sleeps for the given duration.',
        'As an argument you have to pass the time in seconds as a float.',
      ],
    },
    {
      signature: 'void reef_result_int(int value);',
      description: [
        'Sets the given value as the result.',
        'As an argument you have to pass the value as an integer.',
      ],
    },
    {
      signature: 'void reef_result_bytes(uint8_t *ptr, size_t len);',
      description: [
        'Sets the given value as the result.',
        'As an argument you have to pass a pointer to the byte array and the length of the array.',
      ],
    },
    {
      signature: 'void reef_result_string(char *ptr, size_t len);',
      description: [
        'Sets the given value as the result.',
        'As an argument you have to pass a pointer to the char array and the length of the array.',
      ],
    },
    {
      signature: 'void reef_log(char *ptr, size_t num_bytes);',
      description: [
        'Logs the given data.',
        'As an argument you have to pass a pointer to the char array and the number of bytes you want to log.',
      ],
    },
    {
      signature: 'void reef_puts(char *message);',
      description: [
        'Logs a given NULL-terminated string.',
        'As an argument you have to pass a pointer to the char array.',
      ],
    },
    {
      signature: 'void reef_log_int(int val);',
      description: [
        'Logs a given integer value.',
        'As an argument you have to pass the integer value.',
      ],
    },
  ],
};

const librarySection: DocSection = {
  name: 'Library Functions',
  description: [
    'Replacements for functions typically found in the C standard library, which is not available in the Reef environment.',
  ],
  entries: [
    {
      signature: 'unsigned long strlen(const char *ptr);',
      description: [
        'Calculates the length of NULL-terminated string.',
        'As an argument you have to pass a pointer to the char array.',
        'As the result you get the lenghth as a size_t value.',
      ],
    },
    {
      signature: 'char *itoa(int value, char *result, int base);',
      description: [
        'Converts an integer into a string.',
        'As an argument you have to the value to be converted, a buffer to store the result and the base of the integer value.',
        'As the result you get the result buffer.',
      ],
    },
    {
      signature: 'void *malloc(size_t size);',
      description: [
        'Allocate memory of the given size, like the `malloc` function from C standard library.',
        'Use this one instead of the one from malloc.h.',
      ],
    },
    {
      signature: 'void free(void *ptr);',
      description: [
        'Free the memory refernced by the given pointer. Acts like the `free` function from the C standard library.',
        'Use this one instead of the one from malloc.h.',
      ],
    },
  ],
};

export const cStdDoc: DocLang = {
  description: [
    'This is Documentation for the Reef Standard Library for C. It mainly describes all the functions exposed to user submitted code.',
    'All the items for the C Reef std can be imported through `reef.h`.',
  ],
  sections: [mainSection, reefSection, librarySection],
};
