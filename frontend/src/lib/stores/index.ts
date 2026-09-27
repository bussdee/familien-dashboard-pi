import { browser } from '$app/environment';
import { writable } from 'svelte/store';
import type { User } from '$lib/types';

/**
 * The signed-in identity. It mirrors the HttpOnly auth cookie the backend sets
 * — there is deliberately no client-side "switch user", because swapping the
 * name in the UI would not swap the credential behind it.
 */
/**
 * Am Wandtablet ist niemand persönlich angemeldet, aber das Gerät ist es.
 * "device" unterscheidet diesen Familien-Modus von "gar nicht angemeldet":
 * im einen Fall zeigt die App den Familienstand, im anderen den
 * Anmeldebildschirm.
 */
/**
 * Der Familien-Modus wird zusätzlich am Wurzelelement vermerkt, damit die
 * API-Schicht ihn lesen kann, ohne diesen Store zu importieren.
 */
function merken(an: boolean) {
  if (!browser) return;
  if (an) document.documentElement.dataset.familienmodus = 'ja';
  else delete document.documentElement.dataset.familienmodus;
}

function createSession() {
  const { subscribe, set } = writable<{ user: User | null; device: boolean; ready: boolean }>({
    user: null,
    device: false,
    ready: false,
  });

  return {
    subscribe,
    set: (user: User | null) => {
      merken(false);
      set({ user, device: false, ready: true });
    },
    /** Familien-Modus: das Gerät ist angemeldet, die Person nicht. */
    setDevice: () => {
      merken(true);
      set({ user: null, device: true, ready: true });
    },
    clear: () => {
      merken(false);
      set({ user: null, device: false, ready: true });
    },
    markReady: () => set({ user: null, device: false, ready: true }),
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

/**
 * Die Akzentfarbe. Eine Familie will „ihre" Farbe an der Wand haben, und ein
 * Kind erkennt sein Handy daran wieder. Sie gilt pro Gerät, wie hell und
 * dunkel — das Tablet im Flur darf anders aussehen als Papas Handy.
 *
 * Gesetzt wird nur ein Attribut am Wurzelelement; die Farbwerte stehen in
 * app.css, getrennt für hell und dunkel, damit jede Farbe in beiden
 * Stimmungen genug Kontrast behält.
 */
export type Akzent = 'petrol' | 'blau' | 'violett' | 'rose' | 'orange' | 'gruen';

export const AKZENTE: { value: Akzent; label: string; farbe: string }[] = [
  { value: 'petrol', label: 'Petrol', farbe: '#0d9488' },
  { value: 'blau', label: 'Blau', farbe: '#2563eb' },
  { value: 'violett', label: 'Violett', farbe: '#7c3aed' },
  { value: 'rose', label: 'Rose', farbe: '#e11d48' },
  { value: 'orange', label: 'Orange', farbe: '#ea580c' },
  { value: 'gruen', label: 'Grün', farbe: '#16a34a' },
];

function createAkzent() {
  const { subscribe, set } = writable<Akzent>('petrol');

  function apply(wahl: Akzent) {
    if (!browser) return;
    document.documentElement.dataset.akzent = wahl;
    const farbe = AKZENTE.find((a) => a.value === wahl)?.farbe;
    // Auch die Statusleiste des Handys nimmt die Farbe an.
    document
      .querySelectorAll<HTMLMetaElement>('meta[name="theme-color"][media*="light"]')
      .forEach((m) => farbe && (m.content = farbe));
    try {
      localStorage.setItem('akzent', wahl);
    } catch {
      /* private mode */
    }
  }

  return {
    subscribe,
    set: (wahl: Akzent) => {
      set(wahl);
      apply(wahl);
    },
    init: () => {
      if (!browser) return;
      let gespeichert: Akzent = 'petrol';
      try {
        const roh = localStorage.getItem('akzent') as Akzent | null;
        if (roh && AKZENTE.some((a) => a.value === roh)) gespeichert = roh;
      } catch {
        /* private mode */
      }
      set(gespeichert);
      apply(gespeichert);
    },
  };
}

export const akzent = createAkzent();
