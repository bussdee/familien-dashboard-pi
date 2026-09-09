import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import type {
  Activity, BackupFile, CalendarEvent, Chore, DashboardLayout, DeviceStatus,
  DeviceTarget, EventDraft, FileListing, FileUploadResult, Link, LinkDraft,
  MusicBrowse, MusicDirListing, MusicStatus, Note, Photo, PhotoUploadResult,
  Score, ShoppingEvent, ShoppingItem, Track, User, WeatherData, WeatherLocation,
} from '$lib/types';

const BASE = '/api';

/**
 * Liest den Familien-Modus aus dem Wurzelelement. Bewusst nicht über den
 * Store: die Store-Datei importiert diese Datei, das gäbe einen Ringschluss.
 */
function imFamilienModus(): boolean {
  return browser && document.documentElement.dataset.familienmodus === 'ja';
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * request() is the single entry point for every call. A 401 sends the user to
 * the login screen — except when we are already there, which is what made the
 * old client reload itself in a loop.
 */
async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  let res: Response;
  try {
    res = await fetch(`${BASE}${path}`, { ...init, headers, credentials: 'same-origin' });
  } catch {
    throw new ApiError(0, 'Keine Verbindung zum Server');
  }

  if (!res.ok) {
    let message = `Fehler ${res.status}`;
    try {
      const body = await res.json();
      if (body?.message) message = body.message;
    } catch {
      /* a plain-text or empty error body is fine */
    }

    // Ein Wandgerät ist angemeldet — nur eben als niemand. Es auf den
    // Anmeldebildschirm zu werfen, weil ein einzelner persönlicher Endpunkt
    // "nein" sagt, wäre falsch: dann stünde das Tablet im Flur mit einer
    // PIN-Abfrage da.
    if (res.status === 401 && browser && !location.pathname.startsWith('/login') && !imFamilienModus()) {
      void goto('/login');
    }
    throw new ApiError(res.status, message);
  }

  if (res.status === 204 || res.headers.get('Content-Length') === '0') {
    return undefined as T;
  }
  return (await res.json()) as T;
}

const json = (body: unknown): RequestInit => ({ body: JSON.stringify(body) });

export const authApi = {
  /** Public — the login screen needs this before anyone is signed in. */
  roster: () => request<User[]>('/auth/users'),
  login: (userId: number, pin: string) =>
    request<{ user: User }>('/auth/login', { method: 'POST', ...json({ user_id: userId, pin }) }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  /** Liefert entweder die angemeldete Person oder {device:true} am Wandgerät. */
  me: () => request<User | { device: true }>('/auth/me'),
  /** Macht diesen Browser zum Wandgerät (nur Administratoren). */
  enableDevice: () => request<{ device: true }>('/auth/device', { method: 'POST' }),
  /** Beendet den Familien-Modus auf diesem Gerät. */
  disableDevice: () => request<void>('/auth/device', { method: 'DELETE' }),
  changePin: (currentPin: string, newPin: string) =>
    request<void>('/auth/pin', { method: 'POST', ...json({ current_pin: currentPin, new_pin: newPin }) }),
  /** Anyone may change their own name, colour and avatar — but not their role. */
  updateProfile: (data: { name?: string; color?: string; avatar_emoji?: string }) =>
    request<User>('/auth/profile', { method: 'PUT', ...json(data) }),
};

export const prefsApi = {
  layout: () =>
    request<{ key: string; value: DashboardLayout | null }>('/preferences/dashboard.layout'),
  saveLayout: (value: DashboardLayout) =>
    request<void>('/preferences/dashboard.layout', { method: 'PUT', ...json({ value }) }),
};

export const linksApi = {
  list: () =>
    request<{ links: Link[]; categories: string[]; suggested: string[] }>('/links'),
  pinned: () =>
    request<{ links: Link[]; categories: string[]; suggested: string[] }>('/links?pinned=1'),
  create: (draft: LinkDraft) => request<Link>('/links', { method: 'POST', ...json(draft) }),
  update: (id: number, draft: LinkDraft) =>
    request<Link>(`/links/${id}`, { method: 'PUT', ...json(draft) }),
  togglePin: (id: number) => request<Link>(`/links/${id}/pin`, { method: 'POST' }),
  remove: (id: number) => request<void>(`/links/${id}`, { method: 'DELETE' }),
  reorder: (ids: number[]) => request<void>('/links/reorder', { method: 'POST', ...json({ ids }) }),
};

export const adminApi = {
  listUsers: () => request<User[]>('/admin/users'),
  createUser: (data: {
    name: string; color: string; pin: string; role: string; avatar_emoji: string;
    in_rotation?: boolean;
  }) => request<User>('/admin/users', { method: 'POST', ...json(data) }),
  updateUser: (
    id: number,
    data: Partial<{
      name: string; color: string; pin: string; role: string; avatar_emoji: string;
      in_rotation: boolean;
    }>,
  ) => request<void>(`/admin/users/${id}`, { method: 'PUT', ...json(data) }),
  deleteUser: (id: number) => request<void>(`/admin/users/${id}`, { method: 'DELETE' }),

  listDevices: () => request<DeviceTarget[]>('/admin/devices'),
  createDevice: (data: Partial<DeviceTarget>) =>
    request<{ id: number }>('/admin/devices', { method: 'POST', ...json(data) }),
  updateDevice: (id: number, data: Partial<DeviceTarget>) =>
    request<void>(`/admin/devices/${id}`, { method: 'PUT', ...json(data) }),
  deleteDevice: (id: number) => request<void>(`/admin/devices/${id}`, { method: 'DELETE' }),
  reorderDevices: (ids: number[]) =>
    request<void>('/admin/devices/reorder', { method: 'POST', ...json({ ids }) }),
  /** Probe a device before saving it, so the form can report a bad address. */
  testDevice: (data: Partial<DeviceTarget>) =>
    request<{ status: string; latency_ms: number; error: string }>('/admin/devices/test', {
      method: 'POST',
      ...json(data),
    }),
  pointHistory: (userId?: number, limit = 60) =>
    request<Activity[]>(
      `/admin/points?limit=${limit}${userId ? `&user_id=${userId}` : ''}`,
    ),
  /** Positive adds a bonus, negative takes points back off. */
  adjustPoints: (userId: number, points: number, note: string) =>
    request<{ user_id: number; points: number; note: string }>('/admin/points', {
      method: 'POST',
      ...json({ user_id: userId, points, note }),
    }),
  /** Removes one entry; a chore entry also becomes due again. */
  revokePoints: (id: number) => request<void>(`/admin/points/${id}`, { method: 'DELETE' }),
  /** userId 0 resets the whole family. */
  resetPoints: (userId: number) =>
    request<{ removed: number }>('/admin/points/reset', { method: 'POST', ...json({ user_id: userId }) }),

  /** Derselbe Endpunkt wie für die Kachel — die Verwaltung braucht ihn auch. */
  musicStatus: () => request<MusicStatus>('/music/status'),
  rescanMusic: () => request<void>('/music/rescan', { method: 'POST' }),
  /**
   * Die echten Verzeichnisse im eingehängten Ordner. Beim Einrichten ist der
   * Index noch leer, also wird hier das Dateisystem gezeigt, nicht der Index.
   */
  musicFolders: (path = '') =>
    request<MusicDirListing>(`/admin/music/folders?path=${encodeURIComponent(path)}`),
  setMusicDir: (path: string) =>
    request<{ path: string; mount: string }>('/admin/music/dir', {
      method: 'PUT',
      ...json({ path }),
    }),

  listBackups: () => request<BackupFile[]>('/admin/backups'),
  runBackup: () =>
    request<{ database: string; files: string; created: string }>('/admin/backup', { method: 'POST' }),
  downloadUrl: () => `${BASE}/admin/backup/download`,
};

export const weatherApi = {
  get: () => request<WeatherData>('/weather'),
  location: () => request<WeatherLocation>('/weather/location'),
  /** Geocoding runs through the backend so the browser stays off the internet. */
  search: (query: string) =>
    request<WeatherLocation[]>(`/weather/search?q=${encodeURIComponent(query)}`),
  setLocation: (location: WeatherLocation) =>
    request<WeatherLocation>('/admin/weather/location', { method: 'PUT', ...json(location) }),
};

export const calendarApi = {
  events: (days = 30) => request<{ events: CalendarEvent[]; now: string }>(`/calendar?days=${days}`),
  create: (draft: EventDraft) =>
    request<{ id: number }>('/calendar/events', { method: 'POST', ...json(draft) }),
  update: (id: number, draft: EventDraft) =>
    request<{ id: number }>(`/calendar/events/${id}`, { method: 'PUT', ...json(draft) }),
  remove: (id: number) => request<void>(`/calendar/events/${id}`, { method: 'DELETE' }),
};

export const shoppingApi = {
  list: () => request<ShoppingItem[]>('/shopping'),
  create: (data: { name: string; quantity?: string; category?: string }) =>
    request<ShoppingItem>('/shopping', { method: 'POST', ...json(data) }),
  update: (
    id: number,
    data: Partial<{ name: string; quantity: string; category: string; checked: boolean }>,
  ) => request<ShoppingItem>(`/shopping/${id}`, { method: 'PUT', ...json(data) }),
  remove: (id: number) => request<void>(`/shopping/${id}`, { method: 'DELETE' }),
  /** userId nur im Familien-Modus nötig — sonst zählt die eigene Anmeldung. */
  clearChecked: (userId?: number) =>
    request<{ deleted: number; points_awarded: number }>('/shopping/clear-checked', {
      method: 'POST',
      ...json(userId ? { user_id: userId } : {}),
    }),
  /** What the currently ticked-off items are worth, shown before committing. */
  reward: () => request<{ checked_items: number; points_awarded: number }>('/shopping/reward'),
};

export const notesApi = {
  list: () => request<Note[]>('/notes'),
  create: (data: { title: string; content: string; tags?: string[]; pinned?: boolean; shared?: boolean }) =>
    request<Note>('/notes', { method: 'POST', ...json(data) }),
  update: (id: number, data: Partial<{ title: string; content: string; tags: string[]; pinned: boolean }>) =>
    request<Note>(`/notes/${id}`, { method: 'PUT', ...json(data) }),
  remove: (id: number) => request<void>(`/notes/${id}`, { method: 'DELETE' }),
};

export const choresApi = {
  list: () => request<Chore[]>('/chores'),
  create: (data: {
    title: string; description?: string; interval_days?: number; points?: number; assignee_id?: number;
  }) => request<Chore>('/chores', { method: 'POST', ...json(data) }),
  update: (
    id: number,
    data: Partial<{ title: string; description: string; interval_days: number; points: number; assignee_id: number }>,
  ) => request<Chore>(`/chores/${id}`, { method: 'PUT', ...json(data) }),
  remove: (id: number) => request<void>(`/chores/${id}`, { method: 'DELETE' }),
  /** userId nur im Familien-Modus nötig — sonst zählt die eigene Anmeldung. */
  complete: (id: number, userId?: number) =>
    request<{ completed_at: string; next_due_at: string; points_awarded: number; title: string }>(
      `/chores/${id}/complete`,
      { method: 'POST', ...json(userId ? { user_id: userId } : {}) },
    ),
};

export const scoreApi = {
  board: () => request<Score[]>('/scoreboard'),
  history: () => request<Activity[]>('/scoreboard/history'),
};

export const devicesApi = {
  list: () => request<DeviceStatus[]>('/devices'),
};

export const photosApi = {
  list: () => request<{ photos: Photo[]; count: number }>('/photos'),
  url: (name: string) => `${BASE}/photos/${encodeURIComponent(name)}`,
  remove: (name: string) =>
    request<void>(`/photos/${encodeURIComponent(name)}`, { method: 'DELETE' }),

  /**
   * Uploads go through XHR rather than fetch so the widget can show real
   * progress — a phone photo over Wi-Fi is slow enough to need it.
   */
  upload: (files: File[], onProgress?: (percent: number) => void) =>
    uploadMitFortschritt<PhotoUploadResult>('/photos', 'photos', files, onProgress),
};

/**
 * Fotos und Dateien laden auf dieselbe Weise hoch: als Formular, über XHR,
 * mit Fortschritt. fetch() kann keinen Fortschritt melden, und ohne
 * Fortschritt sieht ein Upload über WLAN aus wie ein Absturz.
 */
function uploadMitFortschritt<T>(
  pfad: string,
  feld: string,
  dateien: File[],
  onProgress?: (percent: number) => void,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const body = new FormData();
    for (const datei of dateien) body.append(feld, datei);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${BASE}${pfad}`);
    xhr.withCredentials = true;

    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) {
        onProgress?.(Math.round((event.loaded / event.total) * 100));
      }
    };

    xhr.onload = () => {
      let payload: unknown = null;
      try {
        payload = JSON.parse(xhr.responseText);
      } catch {
        /* an empty or non-JSON body is handled below */
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(payload as T);
      } else {
        const message =
          (payload as { message?: string } | null)?.message ?? `Fehler ${xhr.status}`;
        reject(new ApiError(xhr.status, message));
      }
    };
    xhr.onerror = () => reject(new ApiError(0, 'Upload fehlgeschlagen'));
    xhr.onabort = () => reject(new ApiError(0, 'Upload abgebrochen'));

    xhr.send(body);
  });
}

/**
 * Die Familien-Ablage: Der Administrator lädt hoch, alle laden herunter — am
 * Wandgerät ebenfalls, das sind Familiendateien.
 */
/**
 * Musik. Hören darf jeder, auch das Wandgerät — es ist Familienmusik. Nur das
 * Neu-Einlesen ist Adminsache.
 */
export const musicApi = {
  status: () => request<MusicStatus>('/music/status'),
  browse: (path = '') =>
    request<MusicBrowse>(`/music/browse?path=${encodeURIComponent(path)}`),
  search: (query: string) =>
    request<{ tracks: Track[]; query: string }>(`/music/search?q=${encodeURIComponent(query)}`),
  /**
   * Angesprochen wird über die Kennung, nicht über den Pfad. Damit brauchen
   * Ordner wie „Musik (Kinder)" mit Klammern, Leerzeichen und Umlauten gar
   * keine Kodierung — genau daran wäre die Wiedergabe sonst gescheitert.
   */
  trackUrl: (id: number) => `${BASE}/music/track/${id}`,
  rescan: () => request<void>('/music/rescan', { method: 'POST' }),
};

export const filesApi = {
  list: () => request<FileListing>('/files'),
  /**
   * Bewusst eine Adresse statt eines Aufrufs: Der Browser soll die Datei
   * selbst herunterladen, mit eigener Fortschrittsanzeige und ohne sie vorher
   * komplett in den Arbeitsspeicher zu holen.
   */
  url: (name: string) => `${BASE}/files/${encodeURIComponent(name)}`,
  upload: (files: File[], onProgress?: (percent: number) => void) =>
    uploadMitFortschritt<FileUploadResult>('/files', 'files', files, onProgress),
  remove: (name: string) =>
    request<void>(`/files/${encodeURIComponent(name)}`, { method: 'DELETE' }),
};

/**
 * connectShoppingSocket keeps the family's list in sync. It reconnects with a
 * capped backoff and hands back a disposer, so a component unmount does not
 * leave a reconnect timer running forever (the old version did).
 */
export function connectShoppingSocket(
  onEvent: (event: ShoppingEvent) => void,
  onStatus?: (connected: boolean) => void,
): () => void {
  if (!browser) return () => {};

  let socket: WebSocket | null = null;
  let retry = 0;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  const open = () => {
    if (closed) return;
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    socket = new WebSocket(`${proto}//${location.host}${BASE}/shopping/ws`);

    socket.onopen = () => {
      retry = 0;
      onStatus?.(true);
    };
    socket.onmessage = (event) => {
      try {
        onEvent(JSON.parse(event.data) as ShoppingEvent);
      } catch {
        /* ignore malformed frames */
      }
    };
    socket.onclose = () => {
      onStatus?.(false);
      if (closed) return;
      const delay = Math.min(1000 * 2 ** retry++, 30_000);
      timer = setTimeout(open, delay);
    };
    socket.onerror = () => socket?.close();
  };

  open();

  return () => {
    closed = true;
    if (timer) clearTimeout(timer);
    socket?.close();
  };
}
