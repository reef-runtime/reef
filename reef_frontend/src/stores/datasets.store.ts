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
    const res = await fetch('/api/datasets/upload', {
      method: 'POST',
      body: formData,
    });

    if (res.status !== 200) {
      throw new Error('Failed to upload dataset');
    }

    interface UploadRes {
      id: string;
    }

    const uploadRes: UploadRes = (await res.json()) as IDataset;

    const newDS: IDataset = {
      id: uploadRes.id,
      name: file.name,
      size: file.size,
    };

    set((curr) => ({ datasets: [...curr.datasets, newDS] }));

    return newDS;
  },
}));
