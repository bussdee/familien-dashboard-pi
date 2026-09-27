<script lang="ts">
  import { Check, CircleAlert, Info, X } from 'lucide-svelte';
  import { toasts } from '$lib/stores/toast.svelte';

  const symbol = { info: Info, erfolg: Check, fehler: CircleAlert };
</script>

<!--
  Über der Leiste unten und über der Abspielleiste. aria-live, damit ein
  Bildschirmleser „Auf der Liste" auch vorliest.
-->
<div
  class="pointer-events-none fixed inset-x-0 z-[70] flex flex-col items-center gap-2 px-3"
  style="bottom: calc(var(--unten-leiste, 0px) + 1rem)"
  aria-live="polite"
>
  {#each toasts.liste as t (t.id)}
    {@const Icon = symbol[t.ton]}
    <div
      class="pointer-events-auto flex w-full max-w-md animate-slide-up items-center gap-3 rounded-2xl border border-[color:var(--haarlinie-stark)] bg-card/95 px-4 py-3 text-sm shadow-xl backdrop-blur"
      role="status"
    >
      <span
        class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full
          {t.ton === 'erfolg'
          ? 'bg-success/15 text-success'
          : t.ton === 'fehler'
            ? 'bg-destructive/15 text-destructive'
            : 'bg-primary/15 text-primary'}"
      >
        <Icon class="h-4 w-4" />
      </span>
      <span class="min-w-0 flex-1">{t.text}</span>
      {#if t.aktion}
        <button
          class="shrink-0 rounded-lg px-2 py-1 font-semibold text-primary hover:bg-primary/10"
          onclick={async () => {
            toasts.weg(t.id);
            await t.aktion?.run();
          }}
        >
          {t.aktion.label}
        </button>
      {/if}
      <button
        class="shrink-0 text-muted-foreground hover:text-foreground"
        onclick={() => toasts.weg(t.id)}
        aria-label="Schließen"
      >
        <X class="h-4 w-4" />
      </button>
    </div>
  {/each}
</div>
