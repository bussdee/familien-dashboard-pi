<script lang="ts">
  import { onMount } from 'svelte';
  import { Sun, SunDim } from 'lucide-svelte';
  import { connectShoppingSocket, shoppingApi } from '$lib/api';
  import { connection } from '$lib/stores';
  import ShoppingWidget from '$lib/components/widgets/ShoppingWidget.svelte';
  import type { ShoppingItem } from '$lib/types';

  /**
   * Die Einkaufsliste für den Laden.
   *
   * Auf der Übersicht teilt sie sich den Platz mit zehn anderen Kacheln und
   * ist auf 300 Pixel Höhe begrenzt. Im Markt braucht man das Gegenteil: die
   * ganze Liste, grosse Zeilen, nach Abteilung sortiert — und einen
   * Bildschirm, der nicht alle dreissig Sekunden dunkel wird.
   */
  let items = $state<ShoppingItem[]>([]);
  let geladen = $state(false);
  let fehler = $state('');

  // Bildschirm anlassen. Die Wake Lock API gibt es nicht überall (und nur
  // über HTTPS); wo sie fehlt, verschwindet der Knopf.
  let wachSperre: WakeLockSentinel | null = null;
  let wach = $state(false);
  const wachMoeglich = typeof navigator !== 'undefined' && 'wakeLock' in navigator;

  async function wachUmschalten() {
    if (wach) {
      await wachSperre?.release().catch(() => {});
      wachSperre = null;
      wach = false;
      return;
    }
    try {
      wachSperre = await navigator.wakeLock.request('screen');
      wach = true;
      wachSperre.addEventListener('release', () => (wach = false));
    } catch {
      wach = false;
    }
  }

  onMount(() => {
    shoppingApi
      .list()
      .then((liste) => (items = liste))
      .catch(() => (fehler = 'Die Liste konnte nicht geladen werden.'))
      .finally(() => (geladen = true));

    const trennen = connectShoppingSocket(
      (event) => {
        if (event.action === 'cleared') {
          items = items.filter((i) => !i.checked);
          return;
        }
        if (event.action === 'deleted') {
          items = items.filter((i) => i.id !== event.item.id);
          return;
        }
        const bekannt = items.some((i) => i.id === event.item.id);
        items = bekannt
          ? items.map((i) => (i.id === event.item.id ? event.item : i))
          : [event.item, ...items];
      },
      (live) => connection.setLive(live),
    );

    // Die Sperre fällt beim Wechsel in eine andere App von selbst; zurück in
    // der Liste soll sie wieder gelten.
    const sichtbar = () => {
      if (wach && document.visibilityState === 'visible' && !wachSperre) void wachUmschalten();
    };
    document.addEventListener('visibilitychange', sichtbar);

    return () => {
      trennen();
      connection.setLive(false);
      document.removeEventListener('visibilitychange', sichtbar);
      void wachSperre?.release().catch(() => {});
    };
  });
</script>

<svelte:head><title>Einkaufen · Familien Dashboard</title></svelte:head>

<div class="mx-auto w-full max-w-2xl px-4 py-5 sm:py-7">
  <header class="mb-5 flex items-end justify-between gap-3">
    <div>
      <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {$connection.live ? 'Live mit allen Geräten' : 'Einkaufsliste'}
      </p>
      <h1 class="seiten-titel mt-1">Einkaufen</h1>
    </div>
    {#if wachMoeglich}
      <button
        class="chip shrink-0"
        onclick={wachUmschalten}
        aria-pressed={wach}
        title="Verhindert, dass der Bildschirm im Laden ausgeht"
      >
        {#if wach}<Sun class="h-4 w-4" />{:else}<SunDim class="h-4 w-4" />{/if}
        {wach ? 'Bleibt an' : 'Bildschirm an'}
      </button>
    {/if}
  </header>

  {#if fehler}
    <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
  {/if}

  {#if !geladen}
    <div class="h-64 animate-pulse rounded-2xl bg-muted/40"></div>
  {:else}
    <div class="widget-raster !grid-cols-1">
      <div><ShoppingWidget bind:items voll /></div>
    </div>
  {/if}
</div>
