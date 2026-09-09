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
  }
}

export const diashow = new DiashowStore();
