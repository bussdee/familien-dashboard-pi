<script lang="ts">
  import { X } from 'lucide-svelte';
  import type { Snippet } from 'svelte';

  let {
    open = $bindable(false),
    title,
    onclose,
    children,
  }: {
    open?: boolean;
    title: string;
    onclose?: () => void;
    children: Snippet;
  } = $props();

  let panel = $state<HTMLElement | null>(null);

  function close() {
    open = false;
    onclose?.();
  }

  // Beim Öffnen in das erste Eingabefeld springen — auf dem Handy spart das
  // einen Tipper, am Wandtablet ist sofort klar, wo man schreibt.
  $effect(() => {
    if (!open || !panel) return;
    const first = panel.querySelector<HTMLElement>('input, select, textarea');
    first?.focus();
  });
</script>

<svelte:window
  onkeydown={(e) => {
    if (open && e.key === 'Escape') close();
  }}
/>

{#if open}
  <!-- Auf dem Handy von unten hereingeschoben, am großen Bildschirm mittig.
       Das Formular liegt damit ÜBER der Kachel, statt sie aufzublähen. -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-[60] flex items-end justify-center bg-black/50 p-0 backdrop-blur-sm sm:items-center sm:p-4"
    onclick={(e) => e.target === e.currentTarget && close()}
    role="presentation"
  >
    <div
      bind:this={panel}
      class="safe-bottom max-h-[85vh] w-full max-w-md animate-slide-up overflow-y-auto rounded-t-2xl border border-border bg-card p-5 shadow-xl sm:rounded-2xl"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div class="mb-4 flex items-center justify-between gap-3">
        <h2 class="text-lg font-semibold">{title}</h2>
        <button
          class="touch-target text-muted-foreground transition-colors hover:text-foreground"
          onclick={close}
          aria-label="Schließen"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      {@render children()}
    </div>
  </div>
{/if}
