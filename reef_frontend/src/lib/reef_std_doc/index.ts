export interface DocLang {
  description: string[];
  sections: DocSection[];
}

export interface DocSection {
  name: string;
  description?: string[];
  entries: DocEntry[];
}

export interface DocEntry {
  signature: string;
  description: string[];
  example?: string;
}
