import { create } from 'zustand';

export interface ReefSession {
  id: string;
  isAdmin: boolean;
}

export const useReefSession = create<{
  session: ReefSession;
  setSession: (newSession: ReefSession) => void;
  fetchSession: (token: string | null) => Promise<ReefSession>;
}>((set) => ({
  session: { id: '', isAdmin: false },
  setSession: (session) => set({ session }),
  fetchSession: async (token: string | null) => {
    const res = await fetch('/api/auth', {
      method: 'POST',
      body: JSON.stringify({
        token,
      }),
    });

    if (res.status > 201) {
      console.log(res.statusText);
      return Promise.reject(res.statusText);
    }

    const session: ReefSession = await res.json();

    set({ session });
    return session;
  },
}));
