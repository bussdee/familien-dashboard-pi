import { t } from '$lib/i18n';
import { bonusApi } from '$lib/api';
import { board } from '$lib/stores/scores.svelte';
import type { Tagesaufgabe } from '$lib/types';

/** Die Texte der Tagesaufgaben. Der Server kennt nur Kennung und Ziel. */
export function aufgabenText(id: string): string {
  switch (id) {
    case 'eine':
      return t('Erledige heute eine Aufgabe');
    case 'zwei':
      return t('Erledige heute zwei Aufgaben');
    case 'vormittag':
      return t('Erledige eine Aufgabe vor 12 Uhr');
    case 'punkte15':
      return t('Sammle heute 15 Punkte mit Aufgaben');
  }
  return t('Tagesaufgabe');
}

/**
 * Die Tagesaufgabe eines Kindes. Nachgesehen wird beim Öffnen und immer dann,
 * wenn sich der Punktestand ändert (nach einer erledigten Aufgabe) — der
 * Server schreibt den Bonus gut, sobald sie geschafft ist.
 */
class AufgabeStore {
  stand = $state<Tagesaufgabe | null>(null);
  private laeuft = false;

  async pruefen(userId: number) {
    if (this.laeuft) return;
    this.laeuft = true;
    try {
      const st = await bonusApi.aufgabe();
      this.stand = st.eligible ? st : null;
      if (st.claimed) {
        await board.award(userId, st.points, t('🎯 Tagesaufgabe geschafft: {0}', [aufgabenText(st.id)]));
      }
    } catch {
      /* Beim nächsten Mal. */
    } finally {
      this.laeuft = false;
    }
  }

  vergessen() {
    this.stand = null;
  }
}

export const aufgabe = new AufgabeStore();
