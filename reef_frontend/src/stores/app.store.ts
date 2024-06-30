// import { IJob } from '../types/job';
// import { INode } from '../types/node';
//
// // import { StoreApi, UseBoundStore  create } from 'zustand';
// import { combine } from 'zustand/middleware';
//
// // import { backendConn } from '@/lib/dataProvider';
// import { ReefWebsocket, Topic, TopicKind, allJobs, nodes, singleJob } from '@/lib/websocket';
//
// export interface ReefStore {
//     jobs: IJob[],
//     nodes: INode[],
//     ready: boolean,
// }
//
// const mutations = (setState: any, getState: any) => {
//     const socket = new ReefWebsocket(
//         () => {
//             setState({ ready: true })
//         },
//         () => {
//             setState({ ready: false })
//         },
//     )
//
//     return {
//         actions: {
//             subAllJobs() {
//                 socket.subscribe(allJobs(), (res) => {
//                     setState({
//                         jobs: res.data,
//                     })
//                     return;
//                 })
//             },
//             subSingleJob(jobID: string) {
//                 socket.subscribe(singleJob(jobID), (res) => {
//                     setState({
//                         jobs: [res.data]
//                     })
//                     return;
//                 })
//             },
//             subNodes() {
//                 socket.subscribe(nodes(), (res) => {
//                     setState({
//                         nodes: res.data,
//                     })
//                     return;
//                 })
//             },
//             unsubAll() {
//                 socket.unsubscribeAll()
//             }
//         }
//     }
// }
//
// // const initialState: ReefStore = {
// //     jobs: [],
// //     nodes: [],
// //     ready: false,
// // }
// //
// // let store = create(combine(initialState, mutations))
// //
// // export const useReefData = create((set, get) => {
// //     ...initialState,
// //
// //     execute: async () => {
// //
// //     },
// // })
