<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ChevronDown, ChevronUp, Eye, EyeOff, LayoutGrid, RotateCcw,
  } from 'lucide-svelte';
  import { layout } from '$lib/stores/layout.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  onMount(() => {
    if (!layout.loaded) void layout.load();
  });

  async function reset() {
    const ok = await confirmAction({
      title: 'Ansicht zurücksetzen?',
      message: 'Reihenfolge und Sichtbarkeit gehen auf den Standard zurück.',
      confirmLabel: 'Zurücksetzen',
    });
    if (ok) layout.reset();
  }
</script>

<svelte:head><title>Ansicht anpassen · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 py-5">
  <header class="mb-5">
    <h1 class="flex items-center gap-2 text-2xl font-semibold">
      <LayoutGrid class="h-6 w-6" /> Ansicht anpassen
    </h1>
    <p class="text-sm text-muted-foreground">
      Reihenfolge und Sichtbarkeit der Fenster auf deiner Übersicht. Die
      Einstellung gilt nur für dich.
    </p>
  </header>

  <section class="card mb-4 divide-y divide-border">
    {#each layout.all as widget, index (widget.id)}
      {@const hidden = layout.isHidden(widget.id)}
      <div class="flex items-center gap-3 p-3 {hidden ? 'opacity-55' : ''}">
        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-muted text-xl">
          {widget.emoji}
        </span>

        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium">{widget.label}</p>
          <p class="truncate text-xs text-muted-foreground">
            {hidden ? 'Ausgeblendet' : widget.hint}
          </p>
        </div>

        <div class="flex shrink-0 items-center gap-0.5">
          <button
            class="touch-target text-muted-foreground disabled:opacity-30"
            onclick={() => layout.move(widget.id, -1)}
            disabled={index === 0}
            aria-label="{widget.label} nach oben"
          >
            <ChevronUp class="h-5 w-5" />
          </button>
          <button
            class="touch-target text-muted-foreground disabled:opacity-30"
            onclick={() => layout.move(widget.id, 1)}
            disabled={index === layout.all.length - 1}
            aria-label="{widget.label} nach unten"
          >
            <ChevronDown class="h-5 w-5" />
          </button>
          <button
            class="touch-target {hidden ? 'text-muted-foreground' : 'text-primary'}"
            onclick={() => layout.toggle(widget.id)}
            aria-label={hidden ? `${widget.label} einblenden` : `${widget.label} ausblenden`}
            title={hidden ? 'Einblenden' : 'Ausblenden'}
          >
            {#if hidden}<EyeOff class="h-5 w-5" />{:else}<Eye class="h-5 w-5" />{/if}
          </button>
        </div>
      </div>
    {/each}
  </section>

  <div class="flex items-center justify-between gap-3">
    <p class="text-xs text-muted-foreground">
      {#if layout.saving}
        Wird gespeichert…
      {:else}
        Änderungen werden automatisch gespeichert.
      {/if}
    </p>
    <button class="btn-outline text-sm" onclick={reset}>
      <RotateCcw class="h-4 w-4" /> Zurücksetzen
    </button>
  </div>

  <a href="/" class="btn-primary mt-5 w-full">Zur Übersicht</a>
</div>
