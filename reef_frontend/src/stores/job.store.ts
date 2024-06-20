import { mockJobs } from '@/lib/mockJobs';
import { IJob } from '../types/job';
import { create } from 'zustand';

export const useJobs = create<{
  jobs: IJob[];
  setJobs: (jobs: IJob[]) => void;
}>((set) => ({
  jobs: mockJobs,
  setJobs: (jobs) => set({ jobs }),
}));