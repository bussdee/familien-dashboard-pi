import { t } from '$lib/i18n';
/**
 * Vollbild für die Diashow — der Weg aus dem Browserkorsett.
 *
 * Ein PWA-Manifest mit display:standalone räumt die Adressleiste nur ab,
 * wenn die App wirklich installiert ist. Wer die Diashow im Browser-Tab
 * startet, sah sie sonst weiter. Die Fullscreen-API nimmt sie in jedem Fall:
 * Sie ist ein eigener Benutzerwunsch mit eigener Geste (Klick, Tipp) und
 * verlangt deshalb keine Installation.
 *
 * Safari auf dem iPhone kennt requestFullscreen nur für Video — dort bleibt
 * der Weg über „Zum Home-Bildschirm“ (verfuegbar = false, die Diashow zeigt
 * dann einen Hinweis statt eines toten Knopfs).
 */
type MitVollbild = HTMLElement & { webkitRequestFullscreen?: () => Promise<void> };
type MitVollbildAus = Document & {
  webkitFullscreenElement?: Element | null;
  webkitExitFullscreen?: () => Promise<void>;
};

class Vollbild {
  /** true, solange ein Element im Vollbild steht. */
  aktiv = $state(false);
  /** false, wenn der Browser Vollbild gar nicht anbietet (iPhone Safari). */
  verfuegbar = $state(true);

  constructor() {
    // Auf dem Server gibt es kein document. Die Vorzeichen bleiben neutral,
    // damit die Seite serverseitig nicht schon einen Hinweis rendert, der im
    // Browser sofort wieder verschwindet.
    if (typeof document === 'undefined') return;

    const wurzel = document.documentElement as MitVollbild;
    this.verfuegbar =
      typeof wurzel.requestFullscreen === 'function' ||
      typeof wurzel.webkitRequestFullscreen === 'function';

    // Der Zustand gehört dem Browser: F11, Esc, App-Wechsel — alles kann das
    // Vollbild von aussen beenden. Wir hören mit, statt es zu erraten.
    const abgleichen = () => (this.aktiv = this.element() !== null);
    document.addEventListener('fullscreenchange', abgleichen);
    document.addEventListener('webkitfullscreenchange', abgleichen);
  }

  private element(): Element | null {
    return document.fullscreenElement ?? (document as MitVollbildAus).webkitFullscreenElement ?? null;
  }

  async ein() {
    if (this.aktiv || !this.verfuegbar) return;
    const wurzel = document.documentElement as MitVollbild;
    try {
      if (wurzel.requestFullscreen) await wurzel.requestFullscreen({ navigationUI: 'hide' });
      else if (wurzel.webkitRequestFullscreen) await wurzel.webkitRequestFullscreen();
    } catch {
      // Ohne Benutzergeste oder bei abgelehnter Berechtigung bleibt es beim
      // Browser-Tab — die Diashow läuft trotzdem weiter.
    }
  }

  async aus() {
    if (!this.aktiv) return;
    const doc = document as MitVollbildAus;
    try {
      if (document.exitFullscreen) await document.exitFullscreen();
      else if (doc.webkitExitFullscreen) await doc.webkitExitFullscreen();
    } catch {
      /* schon zu — dann ist ja gut */
    }
  }

  async umschalten() {
    if (this.aktiv) await this.aus();
    else await this.ein();
  }
}

export const vollbild = new Vollbild();
