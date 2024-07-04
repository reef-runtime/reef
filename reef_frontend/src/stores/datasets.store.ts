import { IDataset } from '../types/dataset';
import { create } from 'zustand';

export const useDatasets = create<{
  datasets: IDataset[];
  setDatasets: (datasets: IDataset[]) => void;
}>((set) => ({
  datasets: [],
  setDatasets: (datasets) => set({ datasets }),
}));
