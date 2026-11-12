import { t } from '$lib/i18n';
import { bonusApi } from '$lib/api';
import { board } from '$lib/stores/scores.svelte';
import type { Tagesbonus } from '$lib/types';

/**
 * Der Tagesbonus fürs tägliche Öffnen: Tag 1 → 1 Punkt, Tag 2 → 2 Punkte,
 * und so weiter; fehlt ein Tag, beginnt es wieder bei 1.
 *
 * Abgeholt wird von selbst, sobald ein Kind die App öffnet oder sie wieder in
 * den Vordergrund holt (ein Handy bleibt oft tagelang offen). Der Server
 * entscheidet, ob es heute schon etwas gab — mehrfaches Fragen schadet nicht.
 */
class TagesbonusStore {
  stand = $state<Tagesbonus | null>(null);
  private laeuft = false;

  async abholen(userId: number) {
    if (this.laeuft) return;
    this.laeuft = true;
    try {
      const st = await bonusApi.abholen();
      this.stand = st;
      if (st.claimed && st.points > 0) {
        let label =
          st.streak === 1
            ? t('Tagesbonus! Komm morgen wieder, dann gibt es {0}.', [st.tomorrow])
            : t('Tagesbonus: {0} Tage in Folge! Morgen gibt es {1}.', [st.streak, st.tomorrow]);
        if (st.joker_used) label = t('🛡️ Dein Serienschutz hat gestern gerettet.') + ' ' + label;
        if (st.milestone) {
          label = t('🏅 Meilenstein: {0} Tage in Folge! +{1} Extra-Punkte.', [st.milestone, st.milestone_points ?? 0]);
        }
        await board.award(userId, st.points + (st.milestone_points ?? 0), label);
      }
    } catch {
      /* Ohne Netz gibt es den Bonus beim nächsten Öffnen. */
    } finally {
      this.laeuft = false;
    }
  }

  vergessen() {
    this.stand = null;
  }
}

export const tagesbonus = new TagesbonusStore();
