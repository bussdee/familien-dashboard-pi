/**
 * "Wer war das?" — die Frage am Wandtablet.
 *
 * Im Familien-Modus ist niemand persönlich angemeldet. Wenn jemand eine
 * Aufgabe abhakt, muss trotzdem feststehen, wer die Punkte bekommt. Statt
 * einer Anmeldung genügt ein Tipp aufs eigene Gesicht: die Hürde soll so
 * niedrig sein, dass ein Kind sie im Vorbeigehen nimmt.
 */
export interface WerFrage {
  /** Worum es geht — steht als Untertitel über den Gesichtern. */
  was?: string;
}

class WerWarDasStore {
  frage = $state<WerFrage | null>(null);
  private resolver: ((id: number | null) => void) | null = null;

  /** Löst mit der gewählten Benutzerkennung auf, oder mit null bei Abbruch. */
  ask(frage: WerFrage = {}): Promise<number | null> {
    // Eine zweite Frage bricht die erste ab, statt Dialoge zu stapeln.
    this.resolver?.(null);
    this.frage = frage;
    return new Promise<number | null>((resolve) => {
      this.resolver = resolve;
    });
  }

  answer(id: number | null) {
    this.frage = null;
    const resolve = this.resolver;
    this.resolver = null;
    resolve?.(id);
  }
}

export const werWarDasStore = new WerWarDasStore();

/** Kurzform: `const wer = await werWarDas({ was: 'Müll rausbringen' });` */
export const werWarDas = (frage: WerFrage = {}) => werWarDasStore.ask(frage);
