// // import { mockJobs } from '@/lib/mockJobs';
import { IJob } from '../types/job';
import { create } from 'zustand';
// import { backendConn } from '@/lib/dataProvider';

export const useJobs = create<{
  jobs: IJob[];
  setJobs: (jobs: IJob[]) => void;
}>((set) => {
    // backendConn.subscribe(allJobs(), (data) => {
    //     console.dir(`Updated JOBS: ${data.data}`)
    //     set({jobs: data.data})
    // })

    return {
        jobs: [],
        setJobs: (jobs) => set({ jobs }),
    };
});
