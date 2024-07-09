export type JobLanguage = 'c' | 'rust';

export interface ITemplate {
  id: string;
  name: string;
  code: string;
  dataset: string;
  language: JobLanguage;
}
