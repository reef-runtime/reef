import { DocLang, DocSection } from '.';

const mainSection: DocSection = {
  name: 'User main function',
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

export const cStdDoc: DocLang = {
  description: [
    'This is Documentation for the Reef Standard Library for C. It mainly documents all the functions exposed to user submitted code.',
  ],
  sections: [mainSection],
};
