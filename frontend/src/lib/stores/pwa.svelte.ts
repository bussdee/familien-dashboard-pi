import { browser } from '$app/environment';

/**
 * Alles rund um „als App installiert": ob sich die Seite installieren lässt,
 * ob sie es schon ist, und ob der Offline-Speicher überhaupt arbeiten kann.
 *
 * Zwei Dinge hier sind die Ursache der meisten „PWA geht nicht"-Meldungen:
 *
 *  1. Chrome schickt `beforeinstallprompt` einmal, früh, und danach nie
 *     wieder. Die Seite hat den Hinweis früher erst in einer Komponente
 *     abgefangen, die auf dem Anmeldebildschirm gar nicht existiert — wer
 *     sich dort aufhielt, verpasste das Ereignis für die ganze Sitzung. Das
 *     Modul lauscht deshalb schon beim Laden der App.
 *  2. Installation und Offline-Speicher verlangen eine sichere Herkunft
 *     (https oder localhost). Über http://<IP>:8088 gibt es weder das eine
 *     noch das andere, und die Seite schwieg dazu. Jetzt sagt sie es.
 */
interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

function alsApp(): boolean {
  if (!browser) return false;
  return (
    ['standalone', 'fullscreen', 'minimal-ui'].some((m) => window.matchMedia(`(display-mode: ${m})`).matches) ||
    (navigator as { standalone?: boolean }).standalone === true
  );
}

class PwaStore {
  /** Der Browser hat angeboten, die Seite zu installieren. */
  installierbar = $state(false);
  /** Läuft gerade als installierte App. */
  installiert = $state(alsApp());
  /** https oder localhost — Voraussetzung für Installation und Offline-Speicher. */
  sicher = $state(browser ? window.isSecureContext : true);
  /** Der Browser kennt Service Worker überhaupt. */
  unterstuetzt = $state(browser ? 'serviceWorker' in navigator : true);
  /** Ein Service Worker steuert diese Seite — der Offline-Speicher ist bereit. */
  offlineBereit = $state(false);
  /** iPhone/iPad: dort gibt es kein Installationsfenster, nur den Weg über „Teilen". */
  ios = browser && /iphone|ipad|ipod/i.test(navigator.userAgent);
  /** Safari auf iOS — nur dort lässt sich „Zum Home-Bildschirm" wählen. */
  iosSafari = browser && this.ios && /^((?!chrome|android|crios|fxios).)*safari/i.test(navigator.userAgent);

  private angebot: BeforeInstallPromptEvent | null = null;

  init() {
    if (!browser) return;

    window.addEventListener('beforeinstallprompt', (event) => {
      // Ohne preventDefault zeigt Chrome seine eigene Leiste — und danach
      // lässt sich das Angebot nicht mehr abrufen.
      event.preventDefault();
      this.angebot = event as BeforeInstallPromptEvent;
      this.installierbar = true;
    });

    window.addEventListener('appinstalled', () => {
      this.angebot = null;
      this.installierbar = false;
      this.installiert = true;
    });

    window.matchMedia('(display-mode: standalone)').addEventListener('change', () => {
      this.installiert = alsApp();
    });

    if (this.unterstuetzt) {
      const pruefen = () => (this.offlineBereit = !!navigator.serviceWorker.controller);
      pruefen();
      navigator.serviceWorker.addEventListener('controllerchange', pruefen);
      void navigator.serviceWorker.ready.then(pruefen).catch(() => {});
    }
  }

  /** Öffnet das Installationsfenster des Browsers. */
  async installieren(): Promise<'angenommen' | 'abgelehnt' | 'nicht-moeglich'> {
    if (!this.angebot) return 'nicht-moeglich';
    const angebot = this.angebot;
    this.angebot = null;
    this.installierbar = false;
    await angebot.prompt();
    const { outcome } = await angebot.userChoice;
    return outcome === 'accepted' ? 'angenommen' : 'abgelehnt';
  }

  /**
   * Persönliche Daten aus dem Offline-Speicher nehmen. Beim Anmelden und
   * Abmelden aufgerufen: Der Speicher gehört dem Gerät, nicht der Person.
   */
  leereOfflineDaten() {
    if (!browser || !this.unterstuetzt) return;
    navigator.serviceWorker.controller?.postMessage('clear-api-cache');
  }

  /**
   * Der Notausgang, wenn die App an einer alten Version hängt: Service Worker
   * abmelden, alle Zwischenspeicher leeren, neu laden. Die Daten der Familie
   * liegen auf dem Server und gehen dabei nicht verloren.
   */
  async zuruecksetzen() {
    if (!browser) return;
    try {
      if (this.unterstuetzt) {
        const registrierungen = await navigator.serviceWorker.getRegistrations();
        await Promise.all(registrierungen.map((r) => r.unregister()));
      }
      if ('caches' in window) {
        const namen = await caches.keys();
        await Promise.all(namen.map((n) => caches.delete(n)));
      }
    } finally {
      location.reload();
    }
  }
}

export const pwa = new PwaStore();

// Früh lauschen, nicht erst, wenn eine Komponente erscheint.
pwa.init();
