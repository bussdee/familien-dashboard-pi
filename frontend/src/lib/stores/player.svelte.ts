import { browser } from '$app/environment';
import { musicApi } from '$lib/api';
import type { Track } from '$lib/types';

/**
 * Der Zustand des Abspielers.
 *
 * Das eigentliche <audio>-Element steht im Seitenlayout, nicht in einer
 * Kachel — sonst bräche die Musik ab, sobald jemand auf „Rangliste" tippt.
 * Dieser Speicher ist die Brücke: Er hält Warteschlange und Zustand, das
 * Layout meldet sein Element mit attach() an.
 *
 * Absichtlich NICHT hier: die Hörposition. Das Weiterhören ist für diese
 * Version abbestellt worden.
 */

export type Wiederholung = 'aus' | 'liste' | 'titel';

const LAUTSTAERKE_SCHLUESSEL = 'player.volume';

class PlayerStore {
  /** Die Titel, die gerade gespielt werden — meist ein ganzer Ordner. */
  queue = $state<Track[]>([]);
  index = $state(0);
  playing = $state(false);
  /** Woher die Warteschlange stammt, für die Anzeige in der Leiste. */
  quelle = $state('');

  position = $state(0);
  laenge = $state(0);
  volume = $state(1);
  zufall = $state(false);
  wiederholen = $state<Wiederholung>('aus');
  /** Eine Meldung, wenn eine Datei nicht abgespielt werden kann. */
  fehler = $state('');

  /**
   * Ton startet nie von allein — Browser verbieten das. Nach einem Neustart
   * des Wandtablets muss jemand einmal tippen. Dagegen lässt sich nichts
   * machen, aber die Oberfläche kann es erklären statt stumm zu bleiben.
   */
  brauchtTipp = $state(false);

  private el: HTMLAudioElement | null = null;
  /** Die Reihenfolge bei Zufallswiedergabe; leer, solange sie aus ist. */
  private mischung: number[] = [];

  get current(): Track | null {
    return this.queue[this.index] ?? null;
  }

  get aktiv(): boolean {
    return this.queue.length > 0;
  }

  /** Das Layout meldet sein <audio>-Element hier an. */
  attach(el: HTMLAudioElement) {
    this.el = el;
    el.volume = this.volume;

    el.addEventListener('timeupdate', () => (this.position = el.currentTime));
    el.addEventListener('durationchange', () => {
      // Der Browser kennt die Länge, sobald er die Datei öffnet. Der Server
      // liefert sie nur, wenn sie ausnahmsweise in den ID3-Feldern stand.
      this.laenge = Number.isFinite(el.duration) ? el.duration : 0;
    });
    el.addEventListener('play', () => {
      this.playing = true;
      this.brauchtTipp = false;
      this.medienInfo();
    });
    el.addEventListener('pause', () => (this.playing = false));
    el.addEventListener('ended', () => this.weiter(true));
    el.addEventListener('error', () => {
      this.fehler = this.current
        ? `„${titelVon(this.current)}" lässt sich nicht abspielen.`
        : 'Die Datei lässt sich nicht abspielen.';
      this.playing = false;
    });

    this.medientasten();
  }

  init() {
    if (!browser) return;
    try {
      const gespeichert = localStorage.getItem(LAUTSTAERKE_SCHLUESSEL);
      if (gespeichert !== null) this.setVolume(Number(gespeichert));
    } catch {
      /* privater Modus */
    }
  }

  /**
   * Spielt eine Liste ab dem gewählten Titel. Das ist der einzige Weg, wie
   * Musik startet: Man tippt einen Titel an, und der Rest des Ordners läuft
   * danach weiter — bei einem Hörspiel genau das, was man will.
   */
  spiele(tracks: Track[], start = 0, quelle = '') {
    if (tracks.length === 0) return;
    this.queue = tracks;
    this.quelle = quelle;
    this.index = Math.max(0, Math.min(start, tracks.length - 1));
    this.fehler = '';
    this.mischungNeu();
    void this.laden(true);
  }

  private async laden(abspielen: boolean) {
    const track = this.current;
    if (!this.el || !track) return;

    this.el.src = musicApi.trackUrl(track.id);
    this.position = 0;
    this.laenge = track.duration || 0;
    this.medienInfo();

    if (!abspielen) return;
    try {
      await this.el.play();
    } catch {
      // Der Browser hat die Wiedergabe verweigert, weil sie nicht auf eine
      // Berührung folgte. Kein Fehler — es fehlt nur ein Fingertipp.
      this.brauchtTipp = true;
      this.playing = false;
    }
  }

  async toggle() {
    if (!this.el || !this.current) return;
    if (this.el.paused) {
      try {
        await this.el.play();
      } catch {
        this.brauchtTipp = true;
      }
    } else {
      this.el.pause();
    }
  }

  /**
   * weiter() unterscheidet, ob der Titel von selbst zu Ende ging oder jemand
   * auf „weiter" getippt hat. Nur im ersten Fall darf „Titel wiederholen"
   * greifen — sonst käme man aus einer Endlosschleife nicht mehr heraus.
   */
  weiter(vonSelbst = false) {
    if (this.queue.length === 0) return;

    if (vonSelbst && this.wiederholen === 'titel') {
      void this.laden(true);
      return;
    }

    const naechster = this.naechsterIndex();
    if (naechster === null) {
      this.playing = false;
      if (this.el) this.el.pause();
      return;
    }
    this.index = naechster;
    void this.laden(true);
  }

  private naechsterIndex(): number | null {
    const letzter = this.queue.length - 1;

    if (this.zufall) {
      const stelle = this.mischung.indexOf(this.index);
      if (stelle >= 0 && stelle < this.mischung.length - 1) return this.mischung[stelle + 1];
      if (this.wiederholen === 'liste') {
        this.mischungNeu();
        return this.mischung[0] ?? null;
      }
      return null;
    }

    if (this.index < letzter) return this.index + 1;
    return this.wiederholen === 'liste' ? 0 : null;
  }

  /**
   * Zurück heisst am Anfang eines Titels „voriger Titel", sonst „von vorn".
   * So macht es jeder Abspieler, und bei einem 40-Minuten-Hörspielteil ist
   * das der wichtigere der beiden Fälle.
   */
  zurueck() {
    if (this.position > 3) {
      this.springe(0);
      return;
    }
    if (this.zufall) {
      const stelle = this.mischung.indexOf(this.index);
      if (stelle > 0) {
        this.index = this.mischung[stelle - 1];
        void this.laden(true);
      }
      return;
    }
    if (this.index > 0) {
      this.index -= 1;
      void this.laden(true);
    } else {
      this.springe(0);
    }
  }

  waehle(i: number) {
    if (i < 0 || i >= this.queue.length) return;
    this.index = i;
    void this.laden(true);
  }

  springe(sekunden: number) {
    if (!this.el) return;
    this.el.currentTime = Math.max(0, sekunden);
    this.position = this.el.currentTime;
  }

  /** Vor- und Zurückspulen; bei einem Hörbuch der meistgenutzte Knopf. */
  spule(sekunden: number) {
    if (!this.el) return;
    this.springe(this.el.currentTime + sekunden);
  }

  setVolume(wert: number) {
    const v = Math.max(0, Math.min(1, Number.isFinite(wert) ? wert : 1));
    this.volume = v;
    if (this.el) this.el.volume = v;
    try {
      localStorage.setItem(LAUTSTAERKE_SCHLUESSEL, String(v));
    } catch {
      /* privater Modus */
    }
  }

  zufallUmschalten() {
    this.zufall = !this.zufall;
    this.mischungNeu();
  }

  wiederholungUmschalten() {
    this.wiederholen =
      this.wiederholen === 'aus' ? 'liste' : this.wiederholen === 'liste' ? 'titel' : 'aus';
  }

  /** Beenden räumt auch die Leiste weg. */
  stop() {
    if (this.el) {
      this.el.pause();
      this.el.removeAttribute('src');
      this.el.load();
    }
    this.queue = [];
    this.index = 0;
    this.playing = false;
    this.position = 0;
    this.laenge = 0;
    this.quelle = '';
    this.fehler = '';
    if (browser && 'mediaSession' in navigator) navigator.mediaSession.metadata = null;
  }

  private mischungNeu() {
    if (!this.zufall) {
      this.mischung = [];
      return;
    }
    // Der laufende Titel bleibt vorn, der Rest wird gemischt. Sonst würde das
    // Einschalten der Zufallswiedergabe mitten im Hören umschalten.
    const rest = this.queue.map((_, i) => i).filter((i) => i !== this.index);
    for (let i = rest.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [rest[i], rest[j]] = [rest[j], rest[i]];
    }
    this.mischung = [this.index, ...rest];
  }

  /**
   * Media Session API: Titel und Knöpfe auf dem Sperrbildschirm des Handys
   * und in der Kopfhörer-Bedienung. Wenige Zeilen, grosse Wirkung.
   */
  private medienInfo() {
    if (!browser || !('mediaSession' in navigator)) return;
    const track = this.current;
    if (!track) return;
    navigator.mediaSession.metadata = new MediaMetadata({
      title: titelVon(track),
      artist: track.artist || 'Familien Dashboard',
      album: track.album || letzterOrdner(track.folder),
    });
  }

  private medientasten() {
    if (!browser || !('mediaSession' in navigator)) return;
    const setzen = (aktion: MediaSessionAction, fn: () => void) => {
      try {
        navigator.mediaSession.setActionHandler(aktion, fn);
      } catch {
        // Nicht jeder Browser kennt jede Aktion. Eine unbekannte wirft, und
        // das darf die übrigen nicht mitreissen.
      }
    };
    setzen('play', () => void this.toggle());
    setzen('pause', () => void this.toggle());
    setzen('nexttrack', () => this.weiter());
    setzen('previoustrack', () => this.zurueck());
    setzen('seekforward', () => this.spule(30));
    setzen('seekbackward', () => this.spule(-15));
  }
}

/** Der angezeigte Name: das ID3-Feld, sonst der Dateiname ohne Endung. */
export function titelVon(track: Track): string {
  return track.title || track.filename.replace(/\.[^.]+$/, '');
}

function letzterOrdner(pfad: string): string {
  if (!pfad) return '';
  const i = pfad.lastIndexOf('/');
  return i >= 0 ? pfad.slice(i + 1) : pfad;
}

/** Sekunden als m:ss oder h:mm:ss — Hörspiele werden schnell lang. */
export function zeit(sekunden: number): string {
  if (!Number.isFinite(sekunden) || sekunden <= 0) return '0:00';
  const gesamt = Math.floor(sekunden);
  const s = gesamt % 60;
  const m = Math.floor(gesamt / 60) % 60;
  const h = Math.floor(gesamt / 3600);
  const zwei = (n: number) => String(n).padStart(2, '0');
  return h > 0 ? `${h}:${zwei(m)}:${zwei(s)}` : `${m}:${zwei(s)}`;
}

export const player = new PlayerStore();
