import { t } from '$lib/i18n';
import { scoreApi } from '$lib/api';
import type { Activity, Score } from '$lib/types';

/**
 * One shared leaderboard. The header badge, the ranking widget and the chore
 * list all read from here, so finishing a task updates every one of them at
 * once instead of each refetching on its own schedule.
 */
class ScoreBoard {
  scores = $state<Score[]>([]);
  history = $state<Activity[]>([]);
  loaded = $state(false);

  /** Points just earned, for the celebration overlay. */
  celebration = $state<{ points: number; label: string; levelUp: boolean } | null>(null);

  private timer: ReturnType<typeof setTimeout> | null = null;

  for(userId: number | undefined): Score | null {
    if (userId === undefined) return null;
    return this.scores.find((s) => s.id === userId) ?? null;
  }

  get podium(): Score[] {
    return [...this.scores].sort((a, b) => b.total_points - a.total_points);
  }

  async refresh() {
    const [scores, history] = await Promise.all([
      scoreApi.board(),
      scoreApi.history().catch(() => [] as Activity[]),
    ]);
    this.scores = scores;
    this.history = history;
    this.loaded = true;
  }

  /**
   * Celebrate an award. The level check runs against the score we held before
   * the refresh, so "Level geschafft" only fires on a real crossing.
   */
  async award(userId: number, points: number, label: string) {
    const before = this.for(userId)?.level ?? 1;
    await this.refresh();
    const after = this.for(userId)?.level ?? before;

    this.celebration = { points, label, levelUp: after > before };
    if (this.timer) clearTimeout(this.timer);
    this.timer = setTimeout(() => (this.celebration = null), after > before ? 3600 : 2400);

    if (navigator.vibrate) {
      navigator.vibrate(after > before ? [40, 60, 40, 60, 120] : [30, 40, 30]);
    }
  }

  dismiss() {
    if (this.timer) clearTimeout(this.timer);
    this.celebration = null;
  }
}

export const board = new ScoreBoard();

/** Shared labels so the same wording appears in every place points show up. */
export const sourceLabels: Record<string, { label: string; emoji: string }> = {
  chore: { label: t('Aufgabe'), emoji: '⭐' },
  shopping: { label: t('Einkauf'), emoji: '🛒' },
  bonus: { label: t('Bonus'), emoji: '🎁' },
};

/**
 * Wie ein Eintrag heisst. Eine manuelle Buchung mit Minus ist kein „Bonus" —
 * das stand bis 2.0 so im Verlauf, neben einer roten Zahl.
 */
export function quelle(item: { source: string; points: number }): { label: string; emoji: string } {
  if (item.source === 'bonus' && item.points < 0) return { label: t('Abzug'), emoji: '➖' };
  return sourceLabels[item.source] ?? { label: item.source, emoji: '•' };
}

/** „+10" oder „−10" — mit echtem Minuszeichen statt „+-10". */
export const mitVorzeichen = (punkte: number) =>
  punkte > 0 ? `+${punkte}` : punkte < 0 ? `−${Math.abs(punkte)}` : '0';
