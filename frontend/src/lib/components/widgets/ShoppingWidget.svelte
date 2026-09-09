<script lang="ts">
  import { Check, Plus, ShoppingCart, Sparkles, Trash2, X } from 'lucide-svelte';
  import { ApiError, shoppingApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { werWarDas } from '$lib/stores/werwardas.svelte';
  import type { ShoppingItem } from '$lib/types';

  let {
    items = $bindable([]),
  }: { items: ShoppingItem[] } = $props();

  const categories = [
    '🥦 Obst & Gemüse', '🥛 Milch & Käse', '🍞 Brot & Aufstrich', '🥩 Fleisch & Fisch',
    '🍪 Snacks & Süßes', '🧴 Drogerie', '🏠 Haushalt', '📦 Sonstiges',
  ];

  let name = $state('');
  let quantity = $state('');
  let category = $state('');
  let error = $state('');
  let busy = $state(false);

  const open = $derived(items.filter((i) => !i.checked));
  const done = $derived(items.filter((i) => i.checked));

  // Mirrors the server-side rule, so the button can promise the reward before
  // the trip is finished. The server stays the authority on what is granted.
  const reward = $derived(done.length > 0 ? Math.min(15 + done.length * 2, 80) : 0);

  async function add(event: SubmitEvent) {
    event.preventDefault();
    if (!name.trim() || busy) return;
    busy = true;
    error = '';
    try {
      // The WebSocket echoes the new item back, so the list updates itself.
      await shoppingApi.create({ name: name.trim(), quantity, category });
      name = '';
      quantity = '';
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht hinzufügen';
    } finally {
      busy = false;
    }
  }

  async function toggle(item: ShoppingItem) {
    if (navigator.vibrate) navigator.vibrate(item.checked ? 8 : [10, 25, 10]);
    try {
      await shoppingApi.update(item.id, { checked: !item.checked });
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht speichern';
    }
  }

  async function remove(item: ShoppingItem) {
    try {
      await shoppingApi.remove(item.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht löschen';
    }
  }

  // Implicit form submission is unreliable on some mobile keyboards, so Enter
  // is wired up explicitly from either text field.
  function onEnter(event: KeyboardEvent) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    (event.currentTarget as HTMLElement).closest('form')?.requestSubmit();
  }

  async function clearDone() {
    if (done.length === 0 || busy) return;

    // Fürs Einkaufen gibt es Punkte, also muss auch hier feststehen, wer
    // unterwegs war — am Wandtablet weiß das Gerät es nicht von allein.
    let wer: number | null = $session.user?.id ?? null;
    if ($session.device) {
      wer = await werWarDas({ was: `${done.length} Artikel eingekauft` });
      if (wer === null) return;
    }

    busy = true;
    const count = done.length;
    try {
      const result = await shoppingApi.clearChecked($session.device ? wer! : undefined);
      items = items.filter((i) => !i.checked);
      if (result.points_awarded > 0 && wer !== null) {
        await board.award(
          wer,
          result.points_awarded,
          `Einkauf erledigt · ${count} ${count === 1 ? 'Artikel' : 'Artikel'}`,
        );
      }
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht aufräumen';
    } finally {
      busy = false;
    }
  }
</script>

<section class="flaeche">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <ShoppingCart class="h-5 w-5" /> Einkaufen
      </h2>
      <p class="text-sm text-muted-foreground">
        {open.length} offen · {done.length} im Wagen
      </p>
    </div>
  </header>

  {#if done.length > 0}
    <!-- Finishing the shop is the moment points are earned, so it gets a real
         button rather than a quiet "clear" link. -->
    <button
      class="btn-primary mb-4 w-full justify-between bg-gradient-to-r from-primary to-emerald-500 px-4 py-3"
      onclick={clearDone}
      disabled={busy}
    >
      <span class="flex items-center gap-2">
        <Check class="h-5 w-5" />
        Einkauf abschließen
      </span>
      <span class="flex items-center gap-1 rounded-full bg-white/20 px-2.5 py-1 text-sm font-bold">
        <Sparkles class="h-3.5 w-3.5" />
        +{reward}
      </span>
    </button>
  {/if}

  <form class="mb-4 flex gap-2" onsubmit={add}>
    <input
      class="input flex-1"
      placeholder="Was fehlt?"
      bind:value={name}
      maxlength="80"
      onkeydown={onEnter}
    />
    <input
      class="input w-24 shrink-0"
      placeholder="Menge"
      bind:value={quantity}
      maxlength="20"
      onkeydown={onEnter}
    />
    <button class="btn-primary shrink-0 px-3" disabled={!name.trim() || busy} aria-label="Hinzufügen">
      <Plus class="h-5 w-5" />
    </button>
  </form>

  <select class="input mb-4 text-sm" bind:value={category} aria-label="Kategorie">
    <option value="">Ohne Kategorie</option>
    {#each categories as cat}<option value={cat}>{cat}</option>{/each}
  </select>

  {#if error}
    <p class="mb-3 flex items-center justify-between rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {error}
      <button onclick={() => (error = '')} aria-label="Schließen"><X class="h-4 w-4" /></button>
    </p>
  {/if}

  {#if items.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <ShoppingCart class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Die Liste ist leer</p>
    </div>
  {:else}
    <ul class="scrollbar-thin max-h-[300px] space-y-1.5 overflow-y-auto pr-1">
      {#each open as item (item.id)}
        <li class="group flex items-center gap-3 rounded-lg px-1 py-2.5 transition-colors hover:bg-muted/25">
          <button
            class="touch-target shrink-0"
            onclick={() => toggle(item)}
            aria-label="{item.name} als erledigt markieren"
          >
            <span class="block h-6 w-6 rounded-md border-2 border-border"></span>
          </button>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{item.name}</p>
            <div class="flex flex-wrap items-center gap-2">
              {#if item.quantity}
                <span class="text-xs text-muted-foreground">{item.quantity}</span>
              {/if}
              {#if item.category}
                <span class="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] text-primary">
                  {item.category}
                </span>
              {/if}
            </div>
          </div>
          <span class="row-actions">
            <button
              class="touch-target text-muted-foreground hover:text-destructive"
              onclick={() => remove(item)}
              aria-label="{item.name} löschen"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </span>
        </li>
      {/each}

      {#if done.length > 0}
        <li class="pt-2">
          <p class="mb-1 text-xs uppercase tracking-wider text-muted-foreground">Im Wagen</p>
        </li>
        {#each done as item (item.id)}
          <li class="flex items-center gap-3 rounded-lg bg-success/5 p-2.5">
            <button
              class="touch-target shrink-0"
              onclick={() => toggle(item)}
              aria-label="{item.name} wieder öffnen"
            >
              <span class="flex h-6 w-6 items-center justify-center rounded-md bg-success text-success-foreground">
                <Check class="h-4 w-4" />
              </span>
            </button>
            <span class="flex-1 truncate text-sm text-muted-foreground line-through">
              {item.name}
            </span>
          </li>
        {/each}
      {/if}
    </ul>
  {/if}
</section>
