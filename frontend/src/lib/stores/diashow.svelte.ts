/**
 * Ein winziger gemeinsamer Zustand zwischen der Diashow und dem Seitenlayout.
 *
 * Die Diashow legt sich als Vollbild über alles und blendet ihre Knöpfe aus,
 * solange niemand das Bild berührt. Die Abspielleiste im Seitenlayout soll das
 * mitmachen — sonst stünde sie als heller Streifen unter dem Vollbild. Die
 * beiden können sich nicht direkt sehen, also treffen sie sich hier.
 */
class DiashowStore {
  /** true, solange die Diashow im Vollbild läuft. */
  aktiv = $state(false);
  /** true, während die Bedienelemente eingeblendet sind. */
  wach = $state(false);
  /**
   * true, wenn der Bildschirmschoner die Diashow geöffnet hat und nicht eine
   * Person. Dann reicht ein Tipp, um zurückzukehren — niemand soll sich erst
   * durch Knöpfe tasten, nur weil das Tablet eingeschlafen ist.
   */
  automatisch = $state(false);
  /** Die Seite, auf der der Bildschirmschoner gestartet ist. */
  vorher = $state('');

  /**
   * Beim Betreten ist alles verborgen — genau wie die Knöpfe der Diashow
   * selbst, die ebenfalls erst bei Berührung erscheinen. Stünde die Leiste
   * hier auf "wach", bliebe sie als heller Streifen unter dem Vollbild, bis
   * jemand den Bildschirm anfasst.
   */
  betreten() {
    this.aktiv = true;
    this.wach = false;
  }

  verlassen() {
    this.aktiv = false;
    this.wach = false;
    this.automatisch = false;
    this.vorher = '';
  }

  /** Der Bildschirmschoner startet: merkt sich, wohin es zurückgeht. */
  starten(vorher: string) {
    this.automatisch = true;
    this.vorher = vorher;
  }
}

export const diashow = new DiashowStore();
