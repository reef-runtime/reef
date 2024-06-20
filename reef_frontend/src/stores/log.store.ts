import { mockLogs } from '../lib/mockdata';
import { ILogEntry } from '../types/log';
import { create } from 'zustand';

interface LogState {
  logs: ILogEntry[];
  setLogs: (logs: ILogEntry[]) => void;
}

export const useLogs = create<LogState>((set) => ({
  logs: mockLogs,
  setLogs: (logs) => set({ logs }),
}));
