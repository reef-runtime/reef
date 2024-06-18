import { mockLogs } from "../lib/mockdata";
import { ILogEntry } from "../types/log";
import { create } from 'zustand';

export const useLogs = create((set) => ({
  logs: mockLogs as ILogEntry[],
  setLogs: (logs: ILogEntry[]) => set({ logs }),
}));
