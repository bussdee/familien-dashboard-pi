import { browser } from '$app/environment';
import { setzeElternFreigabe } from '$lib/api';

/**
 * Die Eltern-Freigabe am Wandgerät.
 *
 * Punkte vergeben und Aufgaben bestätigen sind Elternsache. Am Wandtablet ist
 * aber niemand angemeldet — und soll es auch nicht sein. Also fragt das
 * Tablet in diesen beiden Fällen kurz nach der PIN eines Elternteils und hält
 * sie dann zwei Minuten, damit man fünf Aufgaben hintereinander bestätigen
 * kann, ohne fünfmal zu tippen. Danach ist sie vergessen.
 *
 * Auf allen anderen Geräten ist ein Elternteil ohnehin angemeldet; dort sagt
 * brauche() sofort ja.
 */
const DAUER_MS = 2 * 60_000;

function amWandgeraet(): boolean {
  return browser && document.documentElement.dataset.familienmodus === 'ja';
}

class ElternStore {
  /** Offen ist der Dialog, solange hier ein Grund steht. */
  frage = $state<string | null>(null);
  /** Wer gerade freigegeben hat, und bis wann. */
  aktiv = $state<{ id: number; name: string; bis: number } | null>(null);
  /** Sekunden bis zum Ablauf — für die Anzeige oben. */
  rest = $state(0);

  private resolver: ((ok: boolean) => void) | null = null;
  private uhr: ReturnType<typeof setInterval> | null = null;

  /** true, sobald ein Elternteil freigegeben hat (oder gar keine Freigabe nötig ist). */
  brauche(grund: string): Promise<boolean> {
    if (!amWandgeraet()) return Promise.resolve(true);
    if (this.aktiv && Date.now() < this.aktiv.bis) return Promise.resolve(true);
    this.resolver?.(false);
    this.frage = grund;
    return new Promise((resolve) => (this.resolver = resolve));
  }

  /** Vom Dialog nach erfolgreicher Prüfung aufgerufen. */
  freigeben(id: number, name: string, pin: string) {
    const bis = Date.now() + DAUER_MS;
    setzeElternFreigabe({ id, pin, bis });
    this.aktiv = { id, name, bis };
    this.ticken();
    this.frage = null;
    this.resolver?.(true);
    this.resolver = null;
  }

  abbrechen() {
    this.frage = null;
    this.resolver?.(false);
    this.resolver = null;
  }

  /** Vorzeitig sperren — „fertig, Tablet wieder der Familie". */
  sperren() {
    setzeElternFreigabe(null);
    this.aktiv = null;
    this.rest = 0;
    if (this.uhr) clearInterval(this.uhr);
    this.uhr = null;
  }

  private ticken() {
    if (this.uhr) clearInterval(this.uhr);
    const schritt = () => {
      if (!this.aktiv) return;
      this.rest = Math.max(0, Math.ceil((this.aktiv.bis - Date.now()) / 1000));
      if (this.rest === 0) this.sperren();
    };
    schritt();
    this.uhr = setInterval(schritt, 1000);
  }
}

export const eltern = new ElternStore();
