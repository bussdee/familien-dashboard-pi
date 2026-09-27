<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import {
    CalendarPlus, ListPlus, NotebookPen, Plus, ShoppingCart, UtensilsCrossed,
  } from 'lucide-svelte';
  import { ApiError, shoppingApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { layout } from '$lib/stores/layout.svelte';
  import { player } from '$lib/stores/player.svelte';
  import { schnell, type SchnellZiel } from '$lib/stores/schnell.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import type { ShoppingSuggestion } from '$lib/types';
  import Modal from './Modal.svelte';

  /**
   * Das Blatt hinter dem Plus. Was hier steht, ist nach Häufigkeit geordnet:
   * Etwas auf die Einkaufsliste setzen passiert zehnmal öfter als alles
   * andere — deshalb steht das Eingabefeld direkt oben und nicht hinter
   * einem weiteren Tipp.
   */
  let name = $state('');
  let busy = $state(false);
  let vorschlaege = $state<ShoppingSuggestion[]>([]);

  const admin = $derived($session.user?.role === 'admin');
  const aufPfad = $derived($page.url.pathname);

  // Beim Öffnen die Vorschläge holen; beim Tippen gefiltert nachladen.
  let suchTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!schnell.offen) return;
    const q = name.trim();
    if (suchTimer) clearTimeout(suchTimer);
    suchTimer = setTimeout(() => {
      shoppingApi
        .suggestions(q)
        .then((liste) => (vorschlaege = liste.slice(0, 8)))
        .catch(() => (vorschlaege = []));
    }, q ? 150 : 0);
  });

  async function hinzufuegen(eintrag: string, category = '') {
    const text = eintrag.trim();
    if (!text || busy) return;
    busy = true;
    try {
      const item = await shoppingApi.create({ name: text, category });
      name = '';
      vorschlaege = vorschlaege.filter((v) => v.name.toLowerCase() !== text.toLowerCase());
      if (item.existing) {
        toast(`„${item.name}" steht schon auf der Liste`);
        return;
      }
      toast(`„${item.name}" steht auf der Liste`, {
        ton: 'erfolg',
        aktion: {
          label: 'Rückgängig',
          run: async () => {
            await shoppingApi.remove(item.id).catch(() => {});
          },
        },
      });
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht hinzufügen', { ton: 'fehler' });
    } finally {
      busy = false;
    }
  }

  const kachelFuer: Record<SchnellZiel, { id: string; label: string }> = {
    termin: { id: 'calendar', label: 'Kalender' },
    notiz: { id: 'notes', label: 'Notizen' },
    aufgabe: { id: 'chores', label: 'Aufgaben' },
  };

  async function oeffne(ziel: SchnellZiel) {
    schnell.offen = false;
    const kachel = kachelFuer[ziel];
    if (layout.isHidden(kachel.id)) {
      toast(`Die Kachel „${kachel.label}" ist ausgeblendet — unter „Ansicht anpassen" einblenden.`);
      return;
    }
    schnell.anfrage = ziel;
    if (aufPfad !== '/') await goto('/');
    // Die Kachel steht vielleicht weit unten; hinscrollen, damit man sieht,
    // wozu das Formular gehört.
    requestAnimationFrame(() =>
      document.getElementById(kachel.id)?.scrollIntoView({ block: 'center' }),
    );
  }

  async function essenPlanen() {
    schnell.offen = false;
    await goto('/essen?heute=1');
  }

  function onEnter(event: KeyboardEvent) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    void hinzufuegen(name);
  }

  /** Am Rechner öffnet „n" das Blatt — nur, solange man nicht gerade tippt. */
  function taste(event: KeyboardEvent) {
    if (event.key !== 'n' || event.metaKey || event.ctrlKey || event.altKey) return;
    const ziel = event.target as HTMLElement | null;
    if (ziel?.closest('input, textarea, select, [contenteditable="true"]')) return;
    event.preventDefault();
    schnell.offen = true;
  }
</script>

<svelte:window onkeydown={taste} />

<!-- Am grossen Bildschirm gibt es die Leiste unten nicht; dort schwebt das
     Plus rechts unten — über der Abspielleiste, falls Musik läuft. -->
<button
  class="fixed right-6 z-40 hidden h-14 w-14 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-[rgb(var(--akzent-rgb)/0.35)] transition-transform hover:scale-105 active:scale-95 lg:flex
    {player.aktiv ? 'bottom-24' : 'bottom-6'}"
  onclick={() => (schnell.offen = true)}
  aria-label="Schnell hinzufügen"
  title="Schnell hinzufügen (Taste N)"
>
  <Plus class="h-7 w-7" />
</button>

<Modal bind:open={schnell.offen} title="Schnell hinzufügen">
  <div class="space-y-5">
    <div>
      <label for="schnell-einkauf" class="mb-1.5 flex items-center gap-2 text-sm font-medium">
        <ShoppingCart class="h-4 w-4 text-[color:var(--ton-einkaufen)]" /> Auf die Einkaufsliste
      </label>
      <div class="flex gap-2">
        <input
          id="schnell-einkauf"
          class="input flex-1"
          placeholder="Milch, Brot, Waschmittel …"
          bind:value={name}
          maxlength="80"
          onkeydown={onEnter}
          autocomplete="off"
        />
        <button
          class="btn-primary shrink-0 px-4"
          onclick={() => hinzufuegen(name)}
          disabled={!name.trim() || busy}
          aria-label="Hinzufügen"
        >
          <Plus class="h-5 w-5" />
        </button>
      </div>
      {#if vorschlaege.length > 0}
        <div class="mt-2.5 flex flex-wrap gap-1.5" aria-label="Häufig gekauft">
          {#each vorschlaege as v (v.name)}
            <button class="chip" onclick={() => hinzufuegen(v.name, v.category)} disabled={busy}>
              <Plus class="h-3.5 w-3.5 opacity-60" />
              {v.name}
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <div class="grid grid-cols-2 gap-2">
      <button class="schnell-knopf" onclick={() => oeffne('termin')}>
        <span class="kachel-symbol" style="--ton: var(--ton-kalender)"><CalendarPlus class="h-5 w-5" /></span>
        Termin
      </button>
      <button class="schnell-knopf" onclick={essenPlanen}>
        <span class="kachel-symbol" style="--ton: var(--ton-essen)"><UtensilsCrossed class="h-5 w-5" /></span>
        Essen planen
      </button>
      <button class="schnell-knopf" onclick={() => oeffne('notiz')}>
        <span class="kachel-symbol" style="--ton: var(--ton-notizen)"><NotebookPen class="h-5 w-5" /></span>
        Notiz
      </button>
      {#if admin}
        <button class="schnell-knopf" onclick={() => oeffne('aufgabe')}>
          <span class="kachel-symbol" style="--ton: var(--ton-aufgaben)"><ListPlus class="h-5 w-5" /></span>
          Aufgabe
        </button>
      {/if}
    </div>
  </div>
</Modal>

<style>
  .schnell-knopf {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-height: 3.5rem;
    padding: 0.5rem 0.75rem;
    border-radius: 1rem;
    border: 1px solid var(--haarlinie-stark);
    font-size: 0.875rem;
    font-weight: 500;
    text-align: left;
    transition: background-color 0.15s;
  }
  .schnell-knopf:hover {
    background-color: hsl(var(--accent));
  }
</style>
