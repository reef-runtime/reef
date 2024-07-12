import { DocLang, DocSection } from '.';

const mainSection: DocSection = {
  name: 'User main function',
  entries: [
    {
      signature: 'pub fn run(dataset: &[u8]) -> impl Into<ReefResult> {}',
      description: [
        'The entry function for a job written by Reef users. This function must be provided in each job submission because it will be called by wrapper code during execution on a Reef Node.',
        'As argument you are passed slice (pointer) to the dataset which you can safely read from.',
        'You can set the job output by returning any datastructure which can be converted to a supported output type. See `ReefResult` for more information.',
      ],
    },
  ],
};

export const rustStdDoc: DocLang = {
  description: [
    'This is Documentation for the Reef Standard Library for Rust. It mainly documents all the functions exposed to user submitted code.',
  ],
  sections: [mainSection],
};
