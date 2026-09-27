<script lang="ts">
  import { Check, Plus, ShoppingCart, Sparkles, Trash2, X } from 'lucide-svelte';
  import { ApiError, shoppingApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import { werWarDas } from '$lib/stores/werwardas.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import type { ShoppingItem, ShoppingSuggestion } from '$lib/types';

  let {
    items = $bindable([]),
    voll = false,
  }: {
    items: ShoppingItem[];
    /**
     * Die Ansicht für den Laden (/einkaufen): ohne Höhenbegrenzung, nach
     * Kategorien gruppiert — man geht durch den Markt Abteilung für
     * Abteilung, nicht in der Reihenfolge, in der eingetragen wurde.
     */
    voll?: boolean;
  } = $props();

  const categories = [
    '🥦 Obst & Gemüse', '🥛 Milch & Käse', '🍞 Brot & Aufstrich', '🥩 Fleisch & Fisch',
    '🍪 Snacks & Süßes', '🧴 Drogerie', '🏠 Haushalt', '📦 Sonstiges',
  ];

  let name = $state('');
  let quantity = $state('');
  let category = $state('');
  /**
   * Punkt 6: Bisher standen Eingabefeld, Mengenfeld und Kategorieauswahl
   * dauerhaft in der Kachel — drei Zeilen, die den grössten Teil der Fläche
   * belegten, auch wenn gerade niemand etwas eintragen wollte. Notizen und
   * Kalender machen es seit jeher anders: ein Plus in der Kopfzeile klappt
   * das Formular auf. Jetzt hier genauso.
   */
  let showForm = $state(false);

  function startNew() {
    showForm = true;
    name = '';
    quantity = '';
  }

  /**
   * Häufig gekaufte Artikel als Chips unter dem Eingabefeld. Milch tippt man
   * nicht jede Woche neu — man tippt sie an.
   */
  let vorschlaege = $state<ShoppingSuggestion[]>([]);
  let vorschlagTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!showForm) return;
    const q = name.trim();
    // Auch neu laden, wenn sich die Liste ändert: Was draufsteht, soll nicht
    // gleichzeitig als Vorschlag erscheinen.
    void open.length;
    if (vorschlagTimer) clearTimeout(vorschlagTimer);
    vorschlagTimer = setTimeout(() => {
      shoppingApi
        .suggestions(q)
        .then((liste) => (vorschlaege = liste.slice(0, voll ? 12 : 6)))
        .catch(() => (vorschlaege = []));
    }, q ? 150 : 0);
  });

  async function vorschlagNehmen(v: ShoppingSuggestion) {
    try {
      await shoppingApi.create({ name: v.name, category: v.category });
      vorschlaege = vorschlaege.filter((x) => x.name !== v.name);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht hinzufügen';
    }
  }

  async function vorschlagVergessen(v: ShoppingSuggestion) {
    vorschlaege = vorschlaege.filter((x) => x.name !== v.name);
    await shoppingApi.forgetSuggestion(v.name).catch(() => {});
  }

  /** Offene Einträge nach Kategorie, für die Ladenansicht. */
  const gruppen = $derived.by(() => {
    const map = new Map<string, ShoppingItem[]>();
    for (const item of items.filter((i) => !i.checked)) {
      const key = item.category || '📦 Ohne Kategorie';
      map.set(key, [...(map.get(key) ?? []), item]);
    }
    // Reihenfolge wie im Auswahlfeld — ungefähr der Weg durch den Markt.
    const rang = (k: string) => {
      const i = categories.indexOf(k);
      return i === -1 ? categories.length : i;
    };
    return [...map.entries()].sort((a, b) => rang(a[0]) - rang(b[0]));
  });
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
      // Das Formular bleibt bewusst offen — anders als bei Notizen und
      // Kalender. Eine Einkaufsliste füllt man in einem Rutsch: Milch, Brot,
      // Butter. Nach jedem Eintrag erneut auf Plus zu tippen wäre lästig.
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

  // Löschen ohne Rückfrage, dafür mit Rückgängig: Im Laden hält ein Dialog
  // nur auf, ein Vertipper soll trotzdem mit einem Tipp zurückkommen.
  async function remove(item: ShoppingItem) {
    try {
      await shoppingApi.remove(item.id);
      toast(`„${item.name}" gelöscht`, {
        aktion: {
          label: 'Rückgängig',
          run: async () => {
            await shoppingApi
              .create({ name: item.name, quantity: item.quantity, category: item.category })
              .catch(() => {});
          },
        },
      });
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

{#snippet zeileArtikel(item: ShoppingItem)}
        <li class="group flex items-center gap-3 rounded-lg px-1 py-2.5 transition-colors hover:bg-muted/25">
          <button
            class="touch-target shrink-0"
            onclick={() => toggle(item)}
            aria-label="{item.name} als erledigt markieren"
          >
            <span class="block rounded-md border-2 border-border {voll ? 'h-7 w-7' : 'h-6 w-6'}"></span>
          </button>
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium {voll ? 'text-base' : 'text-sm'}">{item.name}</p>
            <div class="flex flex-wrap items-center gap-2">
              {#if item.quantity}
                <span class="text-xs text-muted-foreground">{item.quantity}</span>
              {/if}
              {#if item.category && !voll}
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
{/snippet}

{#snippet zeile()}
  {#if items.length === 0}
    Die Liste ist leer
  {:else}
    {open.length} offen · {done.length} im Wagen
  {/if}
{/snippet}

{#snippet aktionen()}
  <button
    class="btn-primary px-3"
    onclick={() => (showForm ? (showForm = false) : startNew())}
    aria-label={showForm ? 'Abbrechen' : 'Eintrag hinzufügen'}
  >
    {#if showForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
  </button>
{/snippet}

<Kachel ton="var(--ton-einkaufen)" titel="Einkaufen" icon={ShoppingCart} {zeile} {aktionen}>

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

  {#if showForm}
    <form class="mb-4 space-y-2 rounded-xl border border-border p-3" onsubmit={add}>
      <div class="flex gap-2">
        <!-- svelte-ignore a11y_autofocus -->
        <input
          class="input flex-1"
          placeholder="Was fehlt?"
          bind:value={name}
          maxlength="80"
          onkeydown={onEnter}
          autofocus
        />
        <input
          class="input w-24 shrink-0"
          placeholder="Menge"
          bind:value={quantity}
          maxlength="20"
          onkeydown={onEnter}
        />
      </div>
      <select class="input text-sm" bind:value={category} aria-label="Kategorie">
        <option value="">Ohne Kategorie</option>
        {#each categories as cat}<option value={cat}>{cat}</option>{/each}
      </select>
      <button class="btn-primary w-full" disabled={!name.trim() || busy}>
        <Plus class="h-4 w-4" /> Auf die Liste
      </button>
      {#if vorschlaege.length > 0}
        <div class="flex flex-wrap gap-1.5 pt-1" aria-label="Häufig gekauft">
          {#each vorschlaege as v (v.name)}
            <span class="chip !gap-0 !pr-0">
              <button type="button" class="flex items-center gap-1.5" onclick={() => vorschlagNehmen(v)}>
                <Plus class="h-3.5 w-3.5 opacity-60" />
                {v.name}
              </button>
              <button
                type="button"
                class="flex h-9 w-8 items-center justify-center text-muted-foreground hover:text-destructive"
                onclick={() => vorschlagVergessen(v)}
                aria-label="{v.name} nicht mehr vorschlagen"
                title="Nicht mehr vorschlagen"
              >
                <X class="h-3.5 w-3.5" />
              </button>
            </span>
          {/each}
        </div>
      {/if}
    </form>
  {/if}

  {#if error}
    <p class="mb-3 flex items-center justify-between rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
      {error}
      <button onclick={() => (error = '')} aria-label="Schließen"><X class="h-4 w-4" /></button>
    </p>
  {/if}

  {#if items.length === 0}
    <KachelLeer
      icon={ShoppingCart}
      titel="Die Liste ist leer"
      hinweis="Mit + etwas eintragen. Was im Laden abgehakt wird, verschwindet sofort auf allen Geräten."
    />
  {:else}
    <ul class="scrollbar-thin space-y-1.5 pr-1 {voll ? '' : 'max-h-[300px] overflow-y-auto'}">
      {#if voll}
        {#each gruppen as [kategorie, eintraege] (kategorie)}
          <li class="pt-3 first:pt-0">
            <p class="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {kategorie} <span class="font-normal">· {eintraege.length}</span>
            </p>
          </li>
          {#each eintraege as item (item.id)}
            {@render zeileArtikel(item)}
          {/each}
        {/each}
      {:else}
        {#each open as item (item.id)}
          {@render zeileArtikel(item)}
        {/each}
      {/if}

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
</Kachel>
