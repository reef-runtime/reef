import { mockDatasets } from "../lib/mockdata";
import { IDataset } from "../types/dataset";
import { create } from 'zustand';

export const useDatasets = create((set) => ({
  datasets: mockDatasets as IDataset[],
  setDatasets: (datasets: IDataset[]) => set({ datasets }),
}));
