<script lang="ts">
  import { t, dfLocale } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { addDays, format, startOfDay } from 'date-fns';
  import {
    ArrowLeft, BookOpen, CalendarPlus, Clock, ExternalLink, Link2, Maximize2, Minimize2, Pencil,
    Plus, Search, ShoppingCart, Trash2, Users,
  } from 'lucide-svelte';
  import { ApiError, mealsApi, rezepteApi } from '$lib/api';
  import { darfEintragen } from '$lib/stores';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import RezeptFormular from '$lib/components/RezeptFormular.svelte';
  import type { Rezept, RezeptEntwurf } from '$lib/types';

  /**
   * Das Rezeptbuch. Ein Rezept kommt aus einem Link — die meisten Rezeptseiten
   * tragen ihr Rezept maschinenlesbar im HTML, das Dashboard liest es dort
   * aus — oder von Hand. Von hier wandern die Zutaten auf die Einkaufsliste
   * und das Gericht in den Essensplan.
   *
   * Lesen darf jeder, auch das Wandgerät: In der Küche gehört das Rezept an
   * die Wand. Eintragen, ändern und löschen ist Elternsache.
   */
  let rezepte = $state<Rezept[]>([]);
  let laden = $state(true);
  let fehler = $state('');
  let suche = $state('');
  let kategorie = $state('');

  let offen = $state<Rezept | null>(null);
  let oeffnetLaeuft = $state(false);

  const kategorien = $derived([...new Set(rezepte.map((r) => r.category).filter(Boolean))].sort());
  const angezeigt = $derived(
    rezepte.filter((r) => {
      if (kategorie && r.category !== kategorie) return false;
      const q = suche.trim().toLowerCase();
      if (!q) return true;
      return (
        r.title.toLowerCase().includes(q) ||
        r.category.toLowerCase().includes(q) ||
        r.ingredients.some((z) => z.toLowerCase().includes(q))
      );
    }),
  );

  async function laden_() {
    laden = true;
    fehler = '';
    try {
      rezepte = await rezepteApi.list();
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Das Rezeptbuch konnte nicht geladen werden.');
    } finally {
      laden = false;
    }
  }

  // ---- Ein Rezept ansehen: Die Adresse trägt die Nummer (?r=5), damit
  //      „zurück" und ein Lesezeichen auf ein Rezept funktionieren. ----
  async function ansehen(id: number, adresseSetzen = true) {
    oeffnetLaeuft = true;
    try {
      offen = await rezepteApi.get(id);
      if (adresseSetzen) await goto(`/rezepte?r=${id}`, { noScroll: true, keepFocus: true });
    } catch (e) {
      offen = null;
      toast(e instanceof ApiError ? e.message : t('Das Rezept konnte nicht geladen werden.'), { ton: 'fehler' });
    } finally {
      oeffnetLaeuft = false;
    }
  }

  async function schliessen() {
    offen = null;
    kochmodus = false;
    await goto('/rezepte', { noScroll: true, keepFocus: true });
  }

  $effect(() => {
    const id = Number.parseInt($page.url.searchParams.get('r') ?? '', 10);
    if (!id) {
      offen = null;
    } else if (offen?.id !== id && !oeffnetLaeuft) {
      void ansehen(id, false);
    }
  });

  // ---- Hinzufügen ----
  let hinzufuegen = $state(false);
  let weg = $state<'link' | 'hand'>('link');
  let adresse = $state('');
  let holt = $state(false);
  let speichert = $state(false);
  let importFehler = $state('');

  function neu() {
    adresse = '';
    importFehler = '';
    weg = 'link';
    hinzufuegen = true;
  }

  async function holen(event: SubmitEvent) {
    event.preventDefault();
    if (!adresse.trim() || holt) return;
    holt = true;
    importFehler = '';
    try {
      const r = await rezepteApi.importieren(adresse.trim());
      rezepte = [r, ...rezepte];
      hinzufuegen = false;
      toast(t('„{0}“ ist im Rezeptbuch', [r.title]), { ton: 'erfolg' });
      await ansehen(r.id);
    } catch (e) {
      importFehler = e instanceof ApiError ? e.message : t('Das Rezept konnte nicht geholt werden.');
      // Wenn die Seite kein Rezept enthält, ist der nächste sinnvolle Schritt
      // das Formular, nicht eine Sackgasse.
      if (e instanceof ApiError && e.status === 422) weg = 'link';
    } finally {
      holt = false;
    }
  }

  async function vonHand(daten: RezeptEntwurf) {
    speichert = true;
    try {
      const r = await rezepteApi.create(daten);
      rezepte = [r, ...rezepte];
      hinzufuegen = false;
      toast(t('„{0}“ ist im Rezeptbuch', [r.title]), { ton: 'erfolg' });
      await ansehen(r.id);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      speichert = false;
    }
  }

  // ---- Ändern und löschen ----
  let aendern = $state(false);

  async function aenderungSpeichern(daten: RezeptEntwurf) {
    if (!offen) return;
    speichert = true;
    try {
      const r = await rezepteApi.update(offen.id, daten);
      offen = r;
      rezepte = rezepte.map((x) => (x.id === r.id ? { ...r, steps: [] } : x));
      aendern = false;
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      speichert = false;
    }
  }

  async function entfernen() {
    if (!offen) return;
    const r = offen;
    const ok = await confirmAction({
      title: t('„{0}“ löschen?', [r.title]),
      message: t('Das Rezept verschwindet aus dem Rezeptbuch.'),
    });
    if (!ok) return;
    try {
      await rezepteApi.remove(r.id);
      rezepte = rezepte.filter((x) => x.id !== r.id);
      await schliessen();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht löschen'), { ton: 'fehler' });
    }
  }

  // ---- Aus dem Rezept heraus ----
  async function aufDieListe() {
    if (!offen) return;
    try {
      const r = await rezepteApi.toShopping(offen.id);
      toast(
        r.added === 0
          ? t('Alle Zutaten stehen schon auf der Einkaufsliste')
          : t('{0} {1} auf der Einkaufsliste', [r.added, r.added === 1 ? t('Zutat') : t('Zutaten')]),
        { ton: r.added === 0 ? 'info' : 'erfolg' },
      );
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht übernehmen'), { ton: 'fehler' });
    }
  }

  let planen = $state(false);
  const naechsteTage = $derived(Array.from({ length: 7 }, (_, i) => addDays(startOfDay(new Date()), i)));

  async function insEssen(tag: Date) {
    if (!offen) return;
    const day = format(tag, 'yyyy-MM-dd');
    try {
      await mealsApi.save(day, {
        title: offen.title,
        note: offen.source_url ? new URL(offen.source_url).hostname.replace(/^www\./, '') : '',
        ingredients: offen.ingredients,
      });
      planen = false;
      toast(t('„{0}“ steht im Essensplan', [offen.title]), { ton: 'erfolg' });
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht eintragen'), { ton: 'fehler' });
    }
  }

  // ---- Kochmodus: grössere Schrift, der Bildschirm bleibt an ----
  let kochmodus = $state(false);
  let wachschutz: { release: () => Promise<void> } | null = null;

  async function kochmodusUmschalten() {
    kochmodus = !kochmodus;
    if (kochmodus) {
      try {
        if ('wakeLock' in navigator) {
          wachschutz = await (
            navigator as Navigator & { wakeLock: { request: (t: 'screen') => Promise<{ release: () => Promise<void> }> } }
          ).wakeLock.request('screen');
        }
      } catch {
        /* Nicht schlimm: Dann geht der Bildschirm eben irgendwann aus. */
      }
    } else {
      void wachschutz?.release();
      wachschutz = null;
    }
  }

  const minuten = (m: number) =>
    m >= 60 ? `${Math.floor(m / 60)} h${m % 60 ? ` ${m % 60} min` : ''}` : `${m} min`;
  const quellHost = (u: string) => {
    try {
      return new URL(u).hostname.replace(/^www\./, '');
    } catch {
      return '';
    }
  };

  onMount(() => {
    void laden_();
    return () => void wachschutz?.release();
  });
</script>

<svelte:head><title>{t('Rezepte · Familien Dashboard')}</title></svelte:head>

{#if !einstellungen.rezepte}
  <div class="mx-auto max-w-md px-4 py-16 text-center">
    <BookOpen class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
    <p class="font-medium">{t('Das Rezeptbuch ist ausgeschaltet')}</p>
    <p class="mt-1 text-sm text-muted-foreground">{t('Die Eltern können es in der Verwaltung einschalten.')}</p>
  </div>
{:else if offen}
  <!-- ------------------------------------------------------- Ein Rezept -->
  <article class="mx-auto w-full max-w-3xl px-4 py-5 sm:py-7 {kochmodus ? 'text-lg sm:text-xl' : ''}">
    <div class="mb-4 flex items-center justify-between gap-2">
      <button class="btn-ghost -ml-3 gap-1.5 text-sm text-muted-foreground" onclick={schliessen}>
        <ArrowLeft class="h-4 w-4" /> {t('Alle Rezepte')}
      </button>
      <button class="btn-outline px-3 text-sm" onclick={kochmodusUmschalten} aria-pressed={kochmodus}>
        {#if kochmodus}<Minimize2 class="h-4 w-4" />{:else}<Maximize2 class="h-4 w-4" />{/if}
        {kochmodus ? t('Kochmodus beenden') : t('Kochmodus')}
      </button>
    </div>

    {#if offen.has_image}
      <img
        src={rezepteApi.imageUrl(offen.id)}
        alt=""
        class="mb-4 max-h-80 w-full rounded-2xl object-cover"
      />
    {/if}

    <h1 class="seiten-titel">{offen.title}</h1>

    <p class="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
      {#if offen.total_min > 0}<span class="inline-flex items-center gap-1"><Clock class="h-4 w-4" /> {minuten(offen.total_min)}</span>{/if}
      {#if offen.servings}<span class="inline-flex items-center gap-1"><Users class="h-4 w-4" /> {offen.servings}</span>{/if}
      {#if offen.category}<span class="chip !min-h-0 py-0.5 text-xs">{offen.category}</span>{/if}
      {#if offen.source_url}
        <a class="inline-flex items-center gap-1 hover:text-foreground" href={offen.source_url} target="_blank" rel="noopener noreferrer">
          <ExternalLink class="h-4 w-4" /> {quellHost(offen.source_url)}
        </a>
      {/if}
    </p>

    {#if offen.note}
      <p class="mt-3 rounded-xl bg-muted/50 px-4 py-3 text-sm text-muted-foreground">{offen.note}</p>
    {/if}

    <div class="mt-4 flex flex-wrap gap-2">
      {#if offen.ingredients.length > 0 && $darfEintragen}
        <button class="btn-primary text-sm" onclick={aufDieListe}>
          <ShoppingCart class="h-4 w-4" /> {t('Zutaten auf die Einkaufsliste')}
        </button>
      {/if}
      {#if $darfEintragen}
        <button class="btn-outline text-sm" onclick={() => (planen = true)}>
          <CalendarPlus class="h-4 w-4" /> {t('In den Essensplan')}
        </button>
        <button class="btn-ghost px-3 text-sm" onclick={() => (aendern = true)} aria-label={t('Rezept bearbeiten')}>
          <Pencil class="h-4 w-4" />
        </button>
        <button class="btn-ghost px-3 text-sm text-destructive" onclick={entfernen} aria-label={t('Rezept löschen')}>
          <Trash2 class="h-4 w-4" />
        </button>
      {/if}
    </div>

    <div class="mt-6 grid gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
      <section>
        <h2 class="mb-2 text-lg font-semibold">{t('Zutaten')}</h2>
        {#if offen.ingredients.length === 0}
          <p class="text-sm text-muted-foreground">{t('Keine Zutaten eingetragen.')}</p>
        {:else}
          <ul class="space-y-1.5">
            {#each offen.ingredients as zutat, i (i)}
              <li class="flex items-start gap-2.5">
                <input id="z{i}" type="checkbox" class="mt-1.5 h-4 w-4 shrink-0 accent-[hsl(var(--primary))]" />
                <label for="z{i}" class="min-w-0 flex-1 leading-snug has-[:checked]:opacity-50">{zutat}</label>
              </li>
            {/each}
          </ul>
        {/if}
      </section>

      <section>
        <h2 class="mb-2 text-lg font-semibold">{t('Zubereitung')}</h2>
        {#if offen.steps.length === 0}
          <p class="text-sm text-muted-foreground">{t('Keine Schritte eingetragen.')}</p>
        {:else}
          <ol class="space-y-3">
            {#each offen.steps as schritt, i (i)}
              <li class="flex gap-3">
                <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-primary/15 text-sm font-semibold text-primary">{i + 1}</span>
                <p class="min-w-0 flex-1 leading-relaxed">{schritt}</p>
              </li>
            {/each}
          </ol>
        {/if}
      </section>
    </div>
  </article>

  <Modal bind:open={aendern} title={t('Rezept bearbeiten')}>
    <RezeptFormular rezept={offen} {speichert} onSpeichern={aenderungSpeichern} onAbbrechen={() => (aendern = false)} />
  </Modal>

  <Modal bind:open={planen} title={t('Wann soll es das geben?')}>
    <ul class="space-y-1.5">
      {#each naechsteTage as tag (tag.getTime())}
        <li>
          <button class="btn-outline w-full justify-between" onclick={() => insEssen(tag)}>
            <span>{format(tag, 'EEEE', { locale: dfLocale })}</span>
            <span class="text-muted-foreground">{format(tag, t('d. MMM'), { locale: dfLocale })}</span>
          </button>
        </li>
      {/each}
    </ul>
    <p class="mt-3 text-xs text-muted-foreground">{t('Ein Gericht, das an dem Tag schon geplant ist, wird ersetzt.')}</p>
  </Modal>
{:else}
  <!-- ----------------------------------------------------------- Das Buch -->
  <div class="mx-auto w-full max-w-5xl px-4 py-5 sm:py-7">
    <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
          {rezepte.length === 1 ? t('1 Rezept') : t('{0} Rezepte', [rezepte.length])}
        </p>
        <h1 class="seiten-titel mt-1">{t('Rezeptbuch')}</h1>
      </div>
      {#if $darfEintragen}
        <button class="btn-primary" onclick={neu}><Plus class="h-4 w-4" /> {t('Rezept hinzufügen')}</button>
      {/if}
    </header>

    {#if fehler}
      <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
    {/if}

    {#if rezepte.length > 0}
      <div class="mb-4 space-y-2">
        <div class="relative">
          <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input class="input pl-10" type="search" placeholder={t('Suchen: Name, Zutat, Kategorie')} bind:value={suche} aria-label={t('Rezepte durchsuchen')} />
        </div>
        {#if kategorien.length > 0}
          <div class="flex flex-wrap gap-1.5">
            <button class="chip" aria-pressed={kategorie === ''} onclick={() => (kategorie = '')}>{t('Alle')}</button>
            {#each kategorien as k (k)}
              <button class="chip" aria-pressed={kategorie === k} onclick={() => (kategorie = kategorie === k ? '' : k)}>{k}</button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    {#if !laden && rezepte.length === 0 && !fehler}
      <div class="card flex flex-col items-center gap-2 p-10 text-center">
        <BookOpen class="h-10 w-10 text-muted-foreground opacity-40" />
        <p class="font-medium">{t('Noch keine Rezepte')}</p>
        <p class="max-w-sm text-sm text-muted-foreground">
          {$darfEintragen
            ? t('Füge einen Link zu einem Rezept ein, und es landet mit Zutaten, Schritten und Bild im Rezeptbuch.')
            : t('Die Eltern tragen die Rezepte ein.')}
        </p>
        {#if $darfEintragen}
          <button class="btn-primary mt-2" onclick={neu}><Link2 class="h-4 w-4" /> {t('Rezept per Link holen')}</button>
        {/if}
      </div>
    {:else if angezeigt.length === 0 && !laden}
      <p class="py-10 text-center text-sm text-muted-foreground">{t('Nichts gefunden')}</p>
    {:else}
      <ul class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 {laden ? 'opacity-60' : ''}">
        {#each angezeigt as r (r.id)}
          <li>
            <button class="card flex h-full w-full flex-col overflow-hidden text-left transition-colors hover:bg-accent/40" onclick={() => ansehen(r.id)}>
              {#if r.has_image}
                <img src={rezepteApi.imageUrl(r.id)} alt="" loading="lazy" class="h-36 w-full object-cover" />
              {:else}
                <div class="flex h-36 w-full items-center justify-center bg-muted/50 text-4xl">🍽️</div>
              {/if}
              <div class="flex flex-1 flex-col gap-1 p-3.5">
                <p class="font-semibold leading-snug">{r.title}</p>
                <p class="mt-auto flex flex-wrap items-center gap-x-3 gap-y-0.5 pt-1 text-xs text-muted-foreground">
                  {#if r.total_min > 0}<span class="inline-flex items-center gap-1"><Clock class="h-3.5 w-3.5" /> {minuten(r.total_min)}</span>{/if}
                  {#if r.servings}<span class="inline-flex items-center gap-1"><Users class="h-3.5 w-3.5" /> {r.servings}</span>{/if}
                  {#if r.category}<span>{r.category}</span>{/if}
                </p>
              </div>
            </button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>

  <Modal bind:open={hinzufuegen} title={t('Rezept hinzufügen')}>
    <div class="mb-3 flex gap-1 rounded-xl bg-muted/60 p-1" role="tablist">
      <button role="tab" aria-selected={weg === 'link'} class="flex-1 rounded-lg py-2 text-sm font-medium transition-colors {weg === 'link' ? 'bg-card shadow-sm' : 'text-muted-foreground'}" onclick={() => (weg = 'link')}>
        {t('Per Link')}
      </button>
      <button role="tab" aria-selected={weg === 'hand'} class="flex-1 rounded-lg py-2 text-sm font-medium transition-colors {weg === 'hand' ? 'bg-card shadow-sm' : 'text-muted-foreground'}" onclick={() => (weg = 'hand')}>
        {t('Von Hand')}
      </button>
    </div>

    {#if weg === 'link'}
      <form class="space-y-3" onsubmit={holen}>
        <p class="text-sm text-muted-foreground">
          {t('Füge die Adresse einer Rezeptseite ein. Das Dashboard liest Zutaten, Schritte und Bild und legt das Rezept im Buch ab.')}
        </p>
        <input class="input" type="url" inputmode="url" placeholder="https://…" bind:value={adresse} aria-label={t('Adresse des Rezepts')} />
        {#if importFehler}
          <p class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{importFehler}</p>
          <button type="button" class="text-sm text-primary hover:underline" onclick={() => (weg = 'hand')}>
            {t('Stattdessen von Hand eintragen →')}
          </button>
        {/if}
        <button class="btn-primary w-full" disabled={!adresse.trim() || holt}>
          <Link2 class="h-4 w-4" /> {holt ? t('Rezept wird geholt…') : t('Rezept holen')}
        </button>
        <p class="text-xs text-muted-foreground">
          {t('Die Seite wird einmal vom Dashboard-Server abgerufen. Das Bild wird gespeichert, damit später kein Gerät der Familie bei der fremden Seite anfragt.')}
        </p>
      </form>
    {:else}
      <RezeptFormular {speichert} onSpeichern={vonHand} onAbbrechen={() => (hinzufuegen = false)} />
    {/if}
  </Modal>
{/if}
