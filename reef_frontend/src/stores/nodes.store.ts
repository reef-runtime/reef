import { mockNodes } from '../lib/mockdata';
import { INode } from '../types/node';
import { create } from 'zustand';

export const useNodes = create<{
  nodes: INode[];
  setNodes: (nodes: INode[]) => void;
}>((set) => ({
  nodes: mockNodes,
  setNodes: (nodes) => set({ nodes }),
}));
