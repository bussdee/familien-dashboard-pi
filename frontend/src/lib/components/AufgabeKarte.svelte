<script lang="ts">
  import { t } from '$lib/i18n';
  import { Target } from 'lucide-svelte';
  import { aufgabe, aufgabenText } from '$lib/stores/aufgabe.svelte';

  /** Die Tagesaufgabe eines Kindes, unter dem Punkteband der Übersicht. */
  const st = $derived(aufgabe.stand);
  const anteil = $derived(st && st.goal > 0 ? Math.min(100, Math.round((st.progress / st.goal) * 100)) : 0);
</script>

{#if st}
  <section
    class="mb-5 flex items-center gap-3 rounded-2xl border px-4 py-3
      {st.done ? 'border-success/40 bg-success/10' : 'border-[color:var(--haarlinie-stark)] bg-card/60'}"
    aria-label={t('Tagesaufgabe')}
  >
    <span class="kachel-symbol" style="--ton: var(--ton-aufgaben, hsl(var(--primary)))">
      {#if st.done}<span aria-hidden="true">✅</span>{:else}<Target class="h-[18px] w-[18px]" />{/if}
    </span>
    <div class="min-w-0 flex-1">
      <p class="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">{t('Tagesaufgabe')}</p>
      <p class="truncate text-sm font-semibold">{aufgabenText(st.id)}</p>
      {#if !st.done}
        <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
          <div class="h-full rounded-full bg-primary transition-[width] duration-500" style="width: {anteil}%"></div>
        </div>
      {/if}
    </div>
    <span class="shrink-0 text-right text-xs">
      {#if st.done}
        <span class="font-semibold text-success">{t('Geschafft!')}</span>
      {:else}
        <span class="block font-semibold tabular-nums">{st.progress}/{st.goal}</span>
        {#if st.points > 0}<span class="block text-muted-foreground">+{st.points}</span>{/if}
      {/if}
    </span>
  </section>
{/if}
