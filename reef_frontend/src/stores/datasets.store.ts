import { IDataset } from '../types/dataset';
import { create } from 'zustand';

export const useDatasets = create<{
  datasets: IDataset[];
  setDatasets: (datasets: IDataset[]) => void;
  fetchDatasets: () => void;
  uploadDataset: (file: File) => Promise<IDataset>;
}>((set) => ({
  datasets: [],
  setDatasets: (datasets) => set({ datasets }),
  fetchDatasets: async () => {
    const res: IDataset[] = await (await fetch('/api/datasets')).json();
    set({ datasets: res });
  },
  uploadDataset: async (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    const res = await fetch('/api/datasets', {
      method: 'POST',
      body: formData,
    });

    if (res.status !== 200) {
      throw new Error('Failed to upload dataset');
    }

    const newDataset = (await res.json()) as IDataset;
    set((curr) => ({ datasets: [...curr.datasets, newDataset] }));

    return newDataset;
  },
}));
