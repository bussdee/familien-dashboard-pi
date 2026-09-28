/**
 * Schnell hinzufügen — von überall.
 *
 * Der Plus-Knopf unten öffnet ein Blatt mit den häufigsten Handgriffen. Die
 * Einkaufsliste wird direkt dort befüllt. Für Termin, Notiz und Aufgabe
 * springt die App auf die Übersicht und bittet die passende Kachel, ihr
 * eigenes Formular zu öffnen — so gibt es jedes Formular genau einmal.
 */
import type { CalendarEvent } from '$lib/types';

export type SchnellZiel = 'termin' | 'notiz' | 'aufgabe';

class SchnellStore {
  /** Das Blatt mit den Möglichkeiten. */
  offen = $state(false);
  /** Welche Kachel ihr Formular öffnen soll; sie setzt das zurück. */
  anfrage = $state<SchnellZiel | null>(null);

  /**
   * Ein vorhandener Termin, den der Kalender zum Bearbeiten öffnen soll.
   *
   * Der Countdown zeigt Termine, die der Kalender wegen seines kürzeren
   * Fensters nicht kennt — den Geburtstag im August etwa. Er reicht den
   * Termin deshalb selbst herüber, statt ihn den Kalender suchen zu lassen.
   */
  terminBearbeiten = $state<CalendarEvent | null>(null);

  /** Von der Kachel aufgerufen: true, wenn die Anfrage ihr galt. */
  abholen(ziel: SchnellZiel): boolean {
    if (this.anfrage !== ziel) return false;
    this.anfrage = null;
    return true;
  }
}

export const schnell = new SchnellStore();
