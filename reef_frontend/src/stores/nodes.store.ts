import { INode } from '../types/node';
import { create } from 'zustand';
// import { backendConn } from '@/lib/dataProvider';
// import { nodes } from '@/lib/websocket';

export const useNodes = create<{
  nodes: INode[];
  setNodes: (nodes: INode[]) => void;
}>((set) => {
    // backendConn.subscribe(nodes(), (data) => {
    //     console.dir(`Updated Nodes: ${data.data}`)
    //     set({nodes: data.data})
    // })

    return {
        nodes: [],
        setNodes: (nodes) => set({ nodes }),
    }
});
