import { IDataset } from "../types/dataset";
import { create } from 'zustand';

export const useDatasets = create((set) => ({
  datasets: [],
  setDatasets: (datasets: IDataset[]) => set({ datasets }),
}));
