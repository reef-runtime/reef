import { ITemplate } from '../types/template';
import { create } from 'zustand';

export const useTemplates = create<{
  templates: ITemplate[];
  fetchTemplates: () => void;
}>((set) => ({
  templates: [],
  fetchTemplates: async () => {
    const res: ITemplate[] = await (await fetch('/api/templates')).json();
    set({ templates: res });
  },
}));
