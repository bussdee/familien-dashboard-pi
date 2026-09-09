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

/**
 * Die Oberfläche ist unabhängig von hell/dunkel: "nachtlicht" stellt die
 * Fenster ohne Rahmen nebeneinander und trennt sie mit Haarlinien, "glas"
 * legt sie als milchige Scheiben übereinander. Beides funktioniert in beiden
 * Farbstimmungen — hell + Glas ist die freundlichste Kombination.
 */
export type Oberflaeche = 'nachtlicht' | 'glas';

function createOberflaeche() {
  const { subscribe, set } = writable<Oberflaeche>('nachtlicht');

  function apply(wahl: Oberflaeche) {
    if (!browser) return;
    document.documentElement.dataset.oberflaeche = wahl;
    try {
      localStorage.setItem('oberflaeche', wahl);
    } catch {
      /* private mode */
    }
  }

  return {
    subscribe,
    set: (wahl: Oberflaeche) => {
      set(wahl);
      apply(wahl);
    },
    init: () => {
      if (!browser) return;
      let gespeichert: Oberflaeche = 'nachtlicht';
      try {
        gespeichert = (localStorage.getItem('oberflaeche') as Oberflaeche) ?? 'nachtlicht';
      } catch {
        /* private mode */
      }
      set(gespeichert);
      apply(gespeichert);
    },
  };
}

export const oberflaeche = createOberflaeche();

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
