import { prefsApi } from '$lib/api';
import type { DashboardLayout } from '$lib/types';

export interface WidgetMeta {
  id: string;
  label: string;
  emoji: string;
  hint: string;
}

/**
 * The catalogue of dashboard widgets. Order here is the default for someone who
 * has never customised anything: the two lists people act on, then today's
 * information, then the ambient panels.
 */
// Weather is deliberately absent: it sits compactly in the dashboard header
// and opens /wetter for the full forecast, rather than taking a whole tile.
export const WIDGETS: WidgetMeta[] = [
  { id: 'chores', label: 'Aufgaben', emoji: '⭐', hint: 'Was ansteht, mit Punkten' },
  { id: 'shopping', label: 'Einkaufen', emoji: '🛒', hint: 'Gemeinsame Liste, live' },
  { id: 'calendar', label: 'Kalender', emoji: '📅', hint: 'Termine der nächsten Tage' },
  { id: 'links', label: 'Links', emoji: '🔗', hint: 'Angepinnte Lesezeichen' },
  { id: 'countdown', label: 'Countdowns', emoji: '⏰', hint: 'Geburtstage und Ferien' },
  { id: 'notes', label: 'Notizen', emoji: '📝', hint: 'Markdown-Notizen' },
  { id: 'photos', label: 'Foto-Rahmen', emoji: '🖼️', hint: 'Bilder der Familie' },
  { id: 'music', label: 'Musik', emoji: '🎵', hint: 'MP3s und Hörspiele' },
  { id: 'files', label: 'Dateien', emoji: '📎', hint: 'Anleitungen und Formulare' },
  { id: 'devices', label: 'Geräte', emoji: '📱', hint: 'Plex, Kavita und Co.' },
];

const DEFAULT_ORDER = WIDGETS.map((w) => w.id);

class LayoutStore {
  order = $state<string[]>([...DEFAULT_ORDER]);
  hidden = $state<string[]>([]);
  loaded = $state(false);
  /** True while an edit is being written back, so the UI can stay responsive. */
  saving = $state(false);

  private saveTimer: ReturnType<typeof setTimeout> | null = null;

  /** Visible widgets in the person's chosen order. */
  get visible(): WidgetMeta[] {
    return this.order
      .filter((id) => !this.hidden.includes(id))
      .map((id) => WIDGETS.find((w) => w.id === id))
      .filter((w): w is WidgetMeta => w !== undefined);
  }

  get all(): WidgetMeta[] {
    return this.order
      .map((id) => WIDGETS.find((w) => w.id === id))
      .filter((w): w is WidgetMeta => w !== undefined);
  }

  isHidden(id: string) {
    return this.hidden.includes(id);
  }

  async load() {
    try {
      const { value } = await prefsApi.layout();
      if (value) this.apply(value);
    } catch {
      // A missing preference is normal; fall back to the default order.
    } finally {
      this.loaded = true;
    }
  }

  /**
   * apply() reconciles a stored layout with the current widget catalogue, so a
   * widget added in a later version still appears for someone whose saved
   * layout predates it — and a removed one disappears cleanly.
   */
  private apply(layout: DashboardLayout) {
    const known = new Set(DEFAULT_ORDER);
    const stored = (layout.order ?? []).filter((id) => known.has(id));
    const missing = DEFAULT_ORDER.filter((id) => !stored.includes(id));
    this.order = [...stored, ...missing];
    this.hidden = (layout.hidden ?? []).filter((id) => known.has(id));
  }

  toggle(id: string) {
    this.hidden = this.hidden.includes(id)
      ? this.hidden.filter((x) => x !== id)
      : [...this.hidden, id];
    this.persist();
  }

  move(id: string, direction: -1 | 1) {
    const index = this.order.indexOf(id);
    const target = index + direction;
    if (index < 0 || target < 0 || target >= this.order.length) return;

    const next = [...this.order];
    [next[index], next[target]] = [next[target], next[index]];
    this.order = next;
    this.persist();
  }

  moveTo(id: string, position: number) {
    const index = this.order.indexOf(id);
    if (index < 0) return;
    const next = [...this.order];
    next.splice(index, 1);
    next.splice(Math.max(0, Math.min(position, next.length)), 0, id);
    this.order = next;
    this.persist();
  }

  reset() {
    this.order = [...DEFAULT_ORDER];
    this.hidden = [];
    this.persist();
  }

  /** Debounced: dragging through several positions is one write, not five. */
  private persist() {
    if (this.saveTimer) clearTimeout(this.saveTimer);
    this.saving = true;
    this.saveTimer = setTimeout(async () => {
      try {
        await prefsApi.saveLayout({ order: this.order, hidden: this.hidden });
      } catch {
        // Keep the local arrangement; it will be written again on the next edit.
      } finally {
        this.saving = false;
      }
    }, 400);
  }
}

export const layout = new LayoutStore();
