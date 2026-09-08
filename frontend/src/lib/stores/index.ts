import { browser } from '$app/environment';
import { writable } from 'svelte/store';
import type { User } from '$lib/types';

/**
 * The signed-in identity. It mirrors the HttpOnly auth cookie the backend sets
 * — there is deliberately no client-side "switch user", because swapping the
 * name in the UI would not swap the credential behind it.
 */
function createSession() {
  const { subscribe, set } = writable<{ user: User | null; ready: boolean }>({
    user: null,
    ready: false,
  });

  return {
    subscribe,
    set: (user: User | null) => set({ user, ready: true }),
    clear: () => set({ user: null, ready: true }),
    markReady: () => set({ user: null, ready: true }),
  };
}

export const session = createSession();

export type Theme = 'light' | 'dark' | 'system';

function createTheme() {
  const { subscribe, set } = writable<Theme>('system');

  function apply(theme: Theme) {
    if (!browser) return;
    const dark =
      theme === 'dark' ||
      (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
    document.documentElement.classList.toggle('dark', dark);
    try {
      localStorage.setItem('theme', theme);
    } catch {
      /* private mode */
    }
  }

  let current: Theme = 'system';

  return {
    subscribe,
    set: (theme: Theme) => {
      current = theme;
      set(theme);
      apply(theme);
    },
    init: () => {
      if (!browser) return;
      let saved: Theme = 'system';
      try {
        saved = (localStorage.getItem('theme') as Theme) ?? 'system';
      } catch {
        /* private mode */
      }
      current = saved;
      set(saved);
      apply(saved);

      // Follow the OS only while the user has not picked a fixed theme.
      window
        .matchMedia('(prefers-color-scheme: dark)')
        .addEventListener('change', () => current === 'system' && apply('system'));
    },
  };
}

export const theme = createTheme();

function createConnection() {
  const { subscribe, update } = writable({
    online: browser ? navigator.onLine : true,
    live: false,
    lastSync: null as Date | null,
  });

  return {
    subscribe,
    setOnline: (online: boolean) => update((s) => ({ ...s, online })),
    setLive: (live: boolean) => update((s) => ({ ...s, live })),
    synced: () => update((s) => ({ ...s, lastSync: new Date() })),
  };
}

export const connection = createConnection();
