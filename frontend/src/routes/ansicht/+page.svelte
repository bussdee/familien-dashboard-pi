<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import {
    ChevronDown, ChevronUp, Eye, EyeOff, GripVertical, LayoutGrid, RotateCcw,
  } from 'lucide-svelte';
  import { layout } from '$lib/stores/layout.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  onMount(() => {
    if (!layout.loaded) void layout.load();
  });

  /**
   * Ziehen zum Sortieren — mit Finger und Maus gleichermassen, über Pointer
   * Events statt über das HTML-Drag-and-Drop, das auf Touchscreens nicht
   * funktioniert. Die Pfeile bleiben: Wer nicht ziehen kann oder will,
   * sortiert weiter per Tipp oder Tastatur.
   */
  let liste = $state<HTMLElement | null>(null);
  let gezogen = $state<string | null>(null);

  function ziehenStart(event: PointerEvent, id: string) {
    gezogen = id;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    navigator.vibrate?.(8);
  }

  function ziehen(event: PointerEvent) {
    if (!gezogen || !liste) return;
    // Die gezogene Zeile selbst zählt nicht mit: Sie wandert ja gerade, und
    // ihre eigene Mitte würde das Ziel um eins verschieben.
    const andere = [...liste.querySelectorAll<HTMLElement>('[data-kachel]')].filter(
      (z) => z.dataset.kachel !== gezogen,
    );
    // Ziel ist der Platz vor der ersten Zeile, deren Mitte unter dem Finger liegt.
    let ziel = andere.findIndex((z) => {
      const r = z.getBoundingClientRect();
      return event.clientY < r.top + r.height / 2;
    });
    if (ziel === -1) ziel = andere.length;
    if (ziel !== layout.order.indexOf(gezogen)) layout.moveTo(gezogen, ziel);
  }

  function ziehenEnde() {
    gezogen = null;
  }

  async function reset() {
    const ok = await confirmAction({
      title: t('Ansicht zurücksetzen?'),
      message: t('Reihenfolge und Sichtbarkeit gehen auf den Standard zurück.'),
      confirmLabel: t('Zurücksetzen'),
    });
    if (ok) layout.reset();
  }
</script>

<svelte:head><title>{t('Ansicht anpassen · Familien Dashboard')}</title></svelte:head>

<div class="mx-auto max-w-2xl px-4 py-5">
  <header class="mb-5">
    <h1 class="seiten-titel flex items-center gap-2">
      <LayoutGrid class="h-7 w-7" /> {t('Ansicht anpassen')}
    </h1>
    <p class="text-sm text-muted-foreground">
      {t('Reihenfolge und Sichtbarkeit der Fenster auf deiner Übersicht. Am Griff links ziehen oder mit den Pfeilen verschieben. Die Einstellung gilt nur für dich.')}
    </p>
  </header>

  <section class="card mb-4 divide-y divide-border" bind:this={liste}>
    {#each layout.all as widget, index (widget.id)}
      {@const hidden = layout.isHidden(widget.id)}
      <div
        data-kachel={widget.id}
        class="flex items-center gap-2 p-3 transition-colors {hidden ? 'opacity-55' : ''}
          {gezogen === widget.id ? 'relative z-10 rounded-xl bg-primary/10 shadow-lg' : ''}"
      >
        <button
          class="touch-target -ml-1 cursor-grab touch-none text-muted-foreground active:cursor-grabbing"
          onpointerdown={(e) => ziehenStart(e, widget.id)}
          onpointermove={ziehen}
          onpointerup={ziehenEnde}
          onpointercancel={ziehenEnde}
          aria-label="{widget.label} verschieben (ziehen)"
          title={t('Zum Verschieben ziehen')}
        >
          <GripVertical class="h-5 w-5" />
        </button>
        <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-muted text-xl">
          {widget.emoji}
        </span>

        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium">{widget.label}</p>
          <p class="truncate text-xs text-muted-foreground">
            {hidden ? t('Ausgeblendet') : widget.hint}
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
            title={hidden ? t('Einblenden') : t('Ausblenden')}
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
        {t('Wird gespeichert…')}
      {:else}
        {t('Änderungen werden automatisch gespeichert.')}
      {/if}
    </p>
    <button class="btn-outline text-sm" onclick={reset}>
      <RotateCcw class="h-4 w-4" /> {t('Zurücksetzen')}
    </button>
  </div>

  <a href="/" class="btn-primary mt-5 w-full">{t('Zur Übersicht')}</a>
</div>
