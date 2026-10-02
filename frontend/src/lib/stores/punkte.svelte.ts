/**
 * Das Fenster „Punkte vergeben" — von überall zu öffnen: aus der Rangliste,
 * der Verwaltung und dem Plus unten. Bis 2.0 gab es dafür ein einziges
 * Formular tief in der Verwaltung, und genau die war seit 2.0 aus der
 * Kopfleiste verschwunden.
 */
class PunkteStore {
  offen = $state(false);
  /** Wer beim Öffnen schon ausgewählt ist, etwa aus einer Zeile der Rangliste. */
  vorauswahl = $state<number[]>([]);
  /** Ob gutgeschrieben oder abgezogen werden soll. */
  modus = $state<'plus' | 'minus'>('plus');
  /** Zählt jede Buchung und jede Rücknahme — Verläufe laden danach neu. */
  gebucht = $state(0);

  oeffnen(optionen: { fuer?: number[]; modus?: 'plus' | 'minus' } = {}) {
    this.vorauswahl = optionen.fuer ?? [];
    this.modus = optionen.modus ?? 'plus';
    this.offen = true;
  }
}

export const punkte = new PunkteStore();
