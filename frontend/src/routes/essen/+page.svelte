<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { replaceState } from '$app/navigation';
  import {
    ChevronLeft, ChevronRight, Plus, ShoppingCart, Trash2, UtensilsCrossed,
  } from 'lucide-svelte';
  import { addDays, addWeeks, format, isToday, parseISO, startOfWeek } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ApiError, mealsApi } from '$lib/api';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import type { Meal, RecentMeal } from '$lib/types';

  /**
   * Der Essensplan einer Woche. Montag bis Sonntag, weil so eingekauft und
   * gedacht wird — nicht „die nächsten sieben Tage ab heute", bei denen der
   * Montag jeden Tag an eine andere Stelle rutscht.
   */
  let woche = $state(startOfWeek(new Date(), { weekStartsOn: 1 }));
  let meals = $state<Meal[]>([]);
  let recent = $state<RecentMeal[]>([]);
  let laden = $state(true);
  let fehler = $state('');

  const tage = $derived(Array.from({ length: 7 }, (_, i) => addDays(woche, i)));
  const tagKey = (d: Date) => format(d, 'yyyy-MM-dd');
  const essenAm = (d: Date) => meals.find((m) => m.day === tagKey(d)) ?? null;

  const wochenLabel = $derived.by(() => {
    const heute = startOfWeek(new Date(), { weekStartsOn: 1 });
    const diff = Math.round((woche.getTime() - heute.getTime()) / (7 * 86_400_000));
    if (diff === 0) return 'Diese Woche';
    if (diff === 1) return 'Nächste Woche';
    if (diff === -1) return 'Letzte Woche';
    return `${format(woche, 'd. MMM', { locale: de })} – ${format(addDays(woche, 6), 'd. MMM', { locale: de })}`;
  });

  const geplant = $derived(tage.filter((d) => essenAm(d)).length);

  async function laden_() {
    laden = true;
    fehler = '';
    try {
      meals = (await mealsApi.list(tagKey(woche), 7)).meals;
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : 'Der Essensplan konnte nicht geladen werden.';
    } finally {
      laden = false;
    }
  }

  function blaettern(richtung: -1 | 1) {
    woche = addWeeks(woche, richtung);
    void laden_();
  }

  // ---- Bearbeiten ----
  let offen = $state(false);
  let tag = $state('');
  let entwurf = $state({ title: '', note: '', zutaten: '' });
  let speichert = $state(false);
  const vorhanden = $derived(!!meals.find((m) => m.day === tag));

  function bearbeiten(d: Date) {
    tag = tagKey(d);
    const m = essenAm(d);
    entwurf = {
      title: m?.title ?? '',
      note: m?.note ?? '',
      zutaten: (m?.ingredients ?? []).join('\n'),
    };
    offen = true;
  }

  // „Schon mal gekocht": ein Tipp übernimmt Name und Zutaten von damals.
  function uebernehmen(r: RecentMeal) {
    entwurf = { ...entwurf, title: r.title, zutaten: r.ingredients.join('\n') };
  }

  const zutatenListe = () =>
    entwurf.zutaten.split('\n').map((z) => z.trim()).filter(Boolean);

  async function speichern(event?: SubmitEvent, danachEinkaufen = false) {
    event?.preventDefault();
    if (!entwurf.title.trim() || speichert) return;
    speichert = true;
    try {
      const m = await mealsApi.save(tag, {
        title: entwurf.title.trim(),
        note: entwurf.note,
        ingredients: zutatenListe(),
      });
      meals = [...meals.filter((x) => x.day !== tag), m];
      offen = false;
      if (danachEinkaufen) await aufDieListe(tag);
      else toast(`${m.title} eingetragen`, { ton: 'erfolg' });
      void mealsApi.recent().then((r) => (recent = r)).catch(() => {});
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht speichern', { ton: 'fehler' });
    } finally {
      speichert = false;
    }
  }

  async function loeschen() {
    const ok = await confirmAction({
      title: 'Eintrag entfernen?',
      message: 'Der Tag ist danach wieder frei.',
      confirmLabel: 'Entfernen',
    });
    if (!ok) return;
    try {
      await mealsApi.remove(tag);
      meals = meals.filter((m) => m.day !== tag);
      offen = false;
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht löschen', { ton: 'fehler' });
    }
  }

  async function aufDieListe(day: string) {
    try {
      const r = await mealsApi.toShopping(day);
      if (r.added === 0) toast('Alle Zutaten stehen schon auf der Einkaufsliste');
      else
        toast(
          `${r.added} ${r.added === 1 ? 'Zutat' : 'Zutaten'} auf der Einkaufsliste` +
            (r.skipped > 0 ? ` · ${r.skipped} standen schon drauf` : ''),
          { ton: 'erfolg' },
        );
    } catch (e) {
      toast(e instanceof ApiError ? e.message : 'Konnte nicht übernehmen', { ton: 'fehler' });
    }
  }

  onMount(() => {
    void laden_().then(() => {
      // Aus dem Plus unten: gleich den heutigen Tag öffnen.
      if ($page.url.searchParams.has('heute')) {
        bearbeiten(new Date());
        const url = new URL($page.url);
        url.searchParams.delete('heute');
        replaceState(url, {});
      }
    });
    mealsApi.recent().then((r) => (recent = r)).catch(() => {});
  });
</script>

<svelte:head><title>Essensplan · Familien Dashboard</title></svelte:head>

<div class="mx-auto w-full max-w-3xl px-4 py-5 sm:py-7">
  <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
    <div>
      <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {geplant} von 7 Tagen geplant
      </p>
      <h1 class="seiten-titel mt-1">Was gibt's?</h1>
    </div>
    <div class="flex items-center gap-1">
      <button class="btn-ghost rounded-full px-2" onclick={() => blaettern(-1)} aria-label="Woche zurück">
        <ChevronLeft class="h-5 w-5" />
      </button>
      <span class="min-w-[8.5rem] text-center text-sm font-medium">{wochenLabel}</span>
      <button class="btn-ghost rounded-full px-2" onclick={() => blaettern(1)} aria-label="Woche vor">
        <ChevronRight class="h-5 w-5" />
      </button>
    </div>
  </header>

  {#if fehler}
    <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
  {/if}

  <ul class="space-y-2.5 {laden ? 'opacity-60' : ''}">
    {#each tage as d (tagKey(d))}
      {@const m = essenAm(d)}
      {@const heute = isToday(d)}
      {@const vorbei = !heute && d < new Date()}
      <li
        class="card flex items-center gap-3 p-3 sm:gap-4 sm:p-4
          {heute ? '!border-primary/50 ring-1 ring-primary/30' : ''} {vorbei ? 'opacity-60' : ''}"
      >
        <!-- Der Tag als Kalenderblatt: aus drei Metern lesbar. -->
        <div
          class="flex h-14 w-14 shrink-0 flex-col items-center justify-center rounded-2xl
            {heute ? 'bg-primary text-primary-foreground' : 'bg-muted'}"
        >
          <span class="text-[10px] font-semibold uppercase tracking-wider opacity-80">
            {format(d, 'EEE', { locale: de })}
          </span>
          <span class="font-display text-xl leading-none">{format(d, 'd')}</span>
        </div>

        <button class="min-w-0 flex-1 text-left" onclick={() => bearbeiten(d)}>
          {#if m}
            <p class="truncate text-base font-semibold">{m.title}</p>
            <p class="truncate text-xs text-muted-foreground">
              {#if heute}<span class="font-medium text-primary">Heute</span>{/if}
              {#if m.note}{heute ? ' · ' : ''}{m.note}{/if}
              {#if m.ingredients.length > 0}
                {heute || m.note ? ' · ' : ''}{m.ingredients.length}
                {m.ingredients.length === 1 ? 'Zutat' : 'Zutaten'}
              {/if}
            </p>
          {:else}
            <p class="flex items-center gap-1.5 text-sm text-muted-foreground">
              <Plus class="h-4 w-4" /> {heute ? 'Heute noch nichts geplant' : 'Noch nichts geplant'}
            </p>
          {/if}
        </button>

        {#if m && m.ingredients.length > 0}
          <button
            class="touch-target shrink-0 rounded-xl text-muted-foreground hover:bg-accent hover:text-foreground"
            onclick={() => aufDieListe(m.day)}
            aria-label="Zutaten für {m.title} auf die Einkaufsliste"
            title="Zutaten auf die Einkaufsliste"
          >
            <ShoppingCart class="h-5 w-5" />
          </button>
        {/if}
      </li>
    {/each}
  </ul>

  {#if !laden && geplant === 0}
    <p class="mt-6 flex items-start gap-3 rounded-2xl bg-muted/50 p-4 text-sm text-muted-foreground">
      <UtensilsCrossed class="mt-0.5 h-5 w-5 shrink-0" />
      <span>
        Tippe auf einen Tag und trag ein, was es gibt. Mit Zutaten wandert der
        Einkauf dafür mit einem Tipp auf die Einkaufsliste — ohne Doppelte.
      </span>
    </p>
  {/if}
</div>

<Modal
  bind:open={offen}
  title={tag ? format(parseISO(tag), 'EEEE, d. MMMM', { locale: de }) : 'Essen'}
>
  <form class="space-y-3" onsubmit={(e) => speichern(e)}>
    <input class="input" placeholder="Was gibt es? z. B. Spaghetti" bind:value={entwurf.title} maxlength="80" />

    {#if recent.length > 0}
      <div>
        <p class="mb-1.5 text-xs text-muted-foreground">Schon mal gekocht</p>
        <div class="flex flex-wrap gap-1.5">
          {#each recent.slice(0, 10) as r (r.title)}
            <button
              type="button"
              class="chip"
              aria-pressed={entwurf.title === r.title}
              onclick={() => uebernehmen(r)}
            >
              {r.title}
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <input class="input" placeholder="Notiz (optional) — z. B. Oma kommt" bind:value={entwurf.note} maxlength="500" />

    <label class="block text-xs text-muted-foreground">
      Zutaten — eine pro Zeile oder mit Komma getrennt
      <textarea
        class="input mt-1 min-h-[7rem] resize-y"
        placeholder={'Spaghetti\nHackfleisch\nTomaten'}
        bind:value={entwurf.zutaten}
      ></textarea>
    </label>

    <div class="flex flex-wrap gap-2 pt-1">
      {#if vorhanden}
        <button type="button" class="btn-outline px-3 text-destructive" onclick={loeschen} aria-label="Eintrag entfernen">
          <Trash2 class="h-4 w-4" />
        </button>
      {/if}
      <button class="btn-primary flex-1" disabled={!entwurf.title.trim() || speichert}>
        Speichern
      </button>
    </div>
    {#if zutatenListe().length > 0}
      <button
        type="button"
        class="btn-outline w-full"
        disabled={!entwurf.title.trim() || speichert}
        onclick={() => speichern(undefined, true)}
      >
        <ShoppingCart class="h-4 w-4" /> Speichern und Zutaten auf die Liste
      </button>
    {/if}
  </form>
</Modal>
