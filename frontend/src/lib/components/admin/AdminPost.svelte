<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { Mail, Send, Trash2 } from 'lucide-svelte';
  import { ApiError, postApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import type { Brief } from '$lib/types';

  /**
   * Post an die Kinder: ein Lob, eine Überraschung, ein Gute-Nacht-Gruß. Die
   * Nachricht geht erst auf, wenn das Kind das Dashboard das nächste Mal
   * öffnet — auf Wunsch mit ein paar Punkten.
   */
  let gesendet = $state<Brief[]>([]);
  let an = $state(0);
  let text = $state('');
  let bonus = $state(0);
  let sendet = $state(false);

  const empfaenger = $derived(board.scores.filter((p) => p.id !== $session.user?.id));
  const BONI = [0, 5, 10, 20];

  async function laden() {
    try {
      gesendet = (await postApi.liste()).sent ?? [];
    } catch {
      gesendet = [];
    }
  }

  async function senden(event: SubmitEvent) {
    event.preventDefault();
    if (!an || !text.trim() || sendet) return;
    sendet = true;
    try {
      await postApi.schreiben({ to_user: an, text: text.trim(), bonus });
      text = '';
      bonus = 0;
      toast(t('Abgeschickt. Sie geht beim nächsten Öffnen auf.'), { ton: 'erfolg' });
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht senden'), { ton: 'fehler' });
    } finally {
      sendet = false;
    }
  }

  async function zuruecknehmen(b: Brief) {
    try {
      await postApi.zuruecknehmen(b.id);
      await laden();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht zurücknehmen'), { ton: 'fehler' });
    }
  }

  onMount(() => {
    void laden();
    if (board.scores.length === 0) void board.refresh().catch(() => {});
  });
</script>

<section class="card mb-4 p-5">
  <h2 class="mb-1 flex items-center gap-2 font-semibold">
    <Mail class="h-5 w-5" /> {t('Post an die Kinder')}
  </h2>
  <p class="mb-4 text-xs text-muted-foreground">
    {t('Ein Lob, eine Überraschung, ein Gute-Nacht-Gruß: Die Nachricht geht erst auf, wenn sie das Dashboard das nächste Mal öffnet.')}
  </p>

  <form class="space-y-3" onsubmit={senden}>
    <div class="flex flex-wrap gap-1.5" role="group" aria-label={t('An wen')}>
      {#each empfaenger as p (p.id)}
        <button type="button" class="chip" aria-pressed={an === p.id} onclick={() => (an = p.id)}>
          {p.avatar_emoji} {p.name}
        </button>
      {/each}
    </div>
    <textarea class="input min-h-[5rem]" maxlength="280" placeholder={t('z. B. Super gemacht mit dem Zimmer!')} bind:value={text} aria-label={t('Nachricht')}></textarea>
    <div>
      <p class="mb-1 text-xs text-muted-foreground">{t('Bonus beim Öffnen')}</p>
      <div class="flex flex-wrap gap-1.5">
        {#each BONI as b (b)}
          <button type="button" class="chip" aria-pressed={bonus === b} onclick={() => (bonus = b)}>
            {b === 0 ? t('Ohne') : t('{0} Punkte', [b])}
          </button>
        {/each}
      </div>
    </div>
    <button class="btn-primary w-full" disabled={!an || !text.trim() || sendet}>
      <Send class="h-4 w-4" /> {t('Abschicken')}
    </button>
  </form>

  {#if gesendet.length > 0}
    <h3 class="mb-1.5 mt-5 text-xs font-medium uppercase tracking-wider text-muted-foreground">{t('Gesendet')}</h3>
    <ul class="space-y-1.5">
      {#each gesendet as b (b.id)}
        <li class="flex items-start gap-2 rounded-xl bg-muted/40 px-3 py-2 text-sm">
          <span class="min-w-0 flex-1">
            <span class="block text-xs text-muted-foreground">
              {t('an {0}', [b.to_name])} · {b.opened_at ? t('geöffnet') : t('noch zu')}{b.bonus > 0 ? ` · +${b.bonus}` : ''}
            </span>
            <span class="block">{b.text}</span>
          </span>
          {#if !b.opened_at}
            <button class="touch-target -my-2 text-muted-foreground hover:text-destructive" onclick={() => zuruecknehmen(b)} aria-label={t('Zurücknehmen')}>
              <Trash2 class="h-4 w-4" />
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>
