<script lang="ts">
  import { t, dfLocale } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { formatDistanceToNow, parseISO } from 'date-fns';
  import { EyeOff, ExternalLink, Newspaper, Plus, RefreshCw, Rss, Settings2, Trash2, TriangleAlert } from 'lucide-svelte';
  import { ApiError, neuigkeitenApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import type { NewsFeed, NewsItem } from '$lib/types';

  /**
   * Die Neuigkeiten der Familie: Feeds von Zeitungen, Blogs, der Schule. Es
   * gibt Familien-Feeds, die die Eltern pflegen und alle lesen, und
   * persönliche Listen, die nur ihrer Person gehören — beides schalten die
   * Eltern in der Verwaltung ein oder aus, und jede Person kann die
   * Neuigkeiten für sich ausblenden.
   */
  let feeds = $state<NewsFeed[]>([]);
  let items = $state<NewsItem[]>([]);
  let laden = $state(true);
  let fehler = $state('');
  let auswahl = $state(0);
  let aktualisiert = $state(false);

  const admin = $derived($session.user?.role === 'admin');
  const person = $derived(!!$session.user);
  const familienFeeds = $derived(feeds.filter((f) => f.scope === 'familie'));
  const eigeneFeeds = $derived(feeds.filter((f) => f.scope === 'persoenlich'));
  const darfFamilie = $derived(admin);
  const darfPersoenlich = $derived(person && einstellungen.neuigkeitenPersoenlich);

  async function ladeAlles() {
    laden = true;
    fehler = '';
    try {
      [feeds, items] = await Promise.all([neuigkeitenApi.feeds(), neuigkeitenApi.items(60, auswahl || undefined)]);
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Die Neuigkeiten konnten nicht geladen werden.');
    } finally {
      laden = false;
    }
  }

  async function waehlen(id: number) {
    auswahl = auswahl === id ? 0 : id;
    try {
      items = await neuigkeitenApi.items(60, auswahl || undefined);
    } catch {
      /* Die Liste bleibt, wie sie war. */
    }
  }

  async function jetztHolen() {
    aktualisiert = true;
    try {
      feeds = await neuigkeitenApi.refresh();
      items = await neuigkeitenApi.items(60, auswahl || undefined);
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht aktualisieren'), { ton: 'fehler' });
    } finally {
      aktualisiert = false;
    }
  }

  // ---- Feeds verwalten ----
  let verwalten = $state(false);
  let adresse = $state('');
  let titel = $state('');
  let familie = $state(false);
  let sendet = $state(false);
  let formFehler = $state('');

  function neuerFeed() {
    adresse = '';
    titel = '';
    formFehler = '';
    familie = darfFamilie && !darfPersoenlich;
    verwalten = true;
  }

  async function hinzufuegen(event: SubmitEvent) {
    event.preventDefault();
    if (!adresse.trim() || sendet) return;
    sendet = true;
    formFehler = '';
    try {
      feeds = await neuigkeitenApi.create({ url: adresse.trim(), title: titel.trim() || undefined }, familie);
      adresse = '';
      titel = '';
      items = await neuigkeitenApi.items(60, auswahl || undefined);
      toast(t('Feed hinzugefügt'), { ton: 'erfolg' });
    } catch (e) {
      formFehler = e instanceof ApiError ? e.message : t('Der Feed konnte nicht hinzugefügt werden.');
    } finally {
      sendet = false;
    }
  }

  async function entfernen(f: NewsFeed) {
    const ok = await confirmAction({
      title: t('„{0}“ entfernen?', [f.title]),
      message: t('Die Meldungen dieses Feeds verschwinden mit ihm.'),
      confirmLabel: t('Entfernen'),
    });
    if (!ok) return;
    try {
      await neuigkeitenApi.remove(f.id, f.scope === 'familie');
      if (auswahl === f.id) auswahl = 0;
      await ladeAlles();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht entfernen'), { ton: 'fehler' });
    }
  }

  async function ausblenden() {
    const ok = await confirmAction({
      title: t('Neuigkeiten für dich ausblenden?'),
      message: t('Sie verschwinden aus deinem Menü und von deiner Übersicht. Unter Einstellungen holst du sie zurück.'),
      confirmLabel: t('Ausblenden'),
      danger: false,
    });
    if (!ok) return;
    try {
      await einstellungen.setzeNeuigkeitenAus(true);
      location.href = '/';
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    }
  }

  const vor = (iso: string) => formatDistanceToNow(parseISO(iso), { addSuffix: true, locale: dfLocale });
  const host = (u: string) => {
    try {
      return new URL(u).hostname.replace(/^www\./, '');
    } catch {
      return '';
    }
  };

  onMount(() => void ladeAlles());
</script>

<svelte:head><title>{t('Neuigkeiten · Familien Dashboard')}</title></svelte:head>

{#if !einstellungen.neuigkeitenAn}
  <div class="mx-auto max-w-md px-4 py-16 text-center">
    <Newspaper class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
    <p class="font-medium">{t('Die Neuigkeiten sind ausgeschaltet')}</p>
    <p class="mt-1 text-sm text-muted-foreground">{t('Die Eltern können sie in der Verwaltung einschalten.')}</p>
  </div>
{:else}
  <div class="mx-auto w-full max-w-3xl px-4 py-5 sm:py-7">
    <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
          {feeds.length === 1 ? t('1 Quelle') : t('{0} Quellen', [feeds.length])}
        </p>
        <h1 class="seiten-titel mt-1">{t('Neuigkeiten')}</h1>
      </div>
      <div class="flex items-center gap-1">
        {#if admin}
          <button class="btn-ghost px-2" onclick={jetztHolen} disabled={aktualisiert} aria-label={t('Jetzt aktualisieren')} title={t('Jetzt aktualisieren')}>
            <RefreshCw class="h-5 w-5 {aktualisiert ? 'animate-spin' : ''}" />
          </button>
        {/if}
        {#if darfFamilie || darfPersoenlich}
          <button class="btn-outline px-3 text-sm" onclick={() => (verwalten = true)}>
            <Settings2 class="h-4 w-4" /> {t('Quellen')}
          </button>
        {/if}
      </div>
    </header>

    {#if fehler}
      <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
    {/if}

    {#if feeds.length > 1}
      <div class="mb-4 flex flex-wrap gap-1.5">
        <button class="chip" aria-pressed={auswahl === 0} onclick={() => (auswahl = 0, void ladeAlles())}>{t('Alle')}</button>
        {#each feeds as f (f.id)}
          <button class="chip" aria-pressed={auswahl === f.id} onclick={() => waehlen(f.id)}>
            {#if f.scope === 'persoenlich'}<span aria-hidden="true">👤</span>{/if}
            {f.title}
          </button>
        {/each}
      </div>
    {/if}

    {#if !laden && feeds.length === 0}
      <div class="card flex flex-col items-center gap-2 p-10 text-center">
        <Rss class="h-10 w-10 text-muted-foreground opacity-40" />
        <p class="font-medium">{t('Noch keine Quellen')}</p>
        <p class="max-w-sm text-sm text-muted-foreground">
          {darfFamilie || darfPersoenlich
            ? t('Füge die Adresse einer Zeitung, eines Blogs oder der Schule ein. Auch die Adresse der Webseite genügt, wenn sie einen Feed anbietet.')
            : t('Die Eltern tragen die Quellen ein.')}
        </p>
        {#if darfFamilie || darfPersoenlich}
          <button class="btn-primary mt-2" onclick={neuerFeed}><Plus class="h-4 w-4" /> {t('Quelle hinzufügen')}</button>
        {/if}
      </div>
    {:else if !laden && items.length === 0}
      <p class="py-10 text-center text-sm text-muted-foreground">{t('Noch keine Meldungen. Der erste Abruf kann einen Moment dauern.')}</p>
    {:else}
      <ul class="space-y-2.5 {laden ? 'opacity-60' : ''}">
        {#each items as m (m.id)}
          <li class="card p-4">
            <p class="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
              {#if m.scope === 'persoenlich'}<span aria-hidden="true">👤</span>{/if}
              <span class="font-medium">{m.feed_title}</span>
              <span>·</span>
              <time datetime={m.published}>{vor(m.published)}</time>
            </p>
            {#if m.link}
              <a
                class="mt-1 flex items-start gap-1.5 font-semibold leading-snug hover:text-primary"
                href={m.link}
                target="_blank"
                rel="noopener noreferrer"
              >
                <span class="min-w-0 flex-1">{m.title}</span>
                <ExternalLink class="mt-1 h-4 w-4 shrink-0 text-muted-foreground" aria-label={host(m.link)} />
              </a>
            {:else}
              <p class="mt-1 font-semibold leading-snug">{m.title}</p>
            {/if}
            {#if m.summary}
              <p class="mt-1.5 line-clamp-3 text-sm text-muted-foreground">{m.summary}</p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    {#if person}
      <button class="mt-8 flex w-full items-center justify-center gap-2 rounded-xl py-3 text-sm text-muted-foreground transition-colors hover:bg-accent" onclick={ausblenden}>
        <EyeOff class="h-4 w-4" /> {t('Neuigkeiten für mich ausblenden')}
      </button>
    {/if}
  </div>

  <Modal bind:open={verwalten} title={t('Quellen')}>
    <div class="space-y-4">
      {#each [{ titel: t('Für die Familie'), liste: familienFeeds, schreiben: darfFamilie }, { titel: t('Nur für mich'), liste: eigeneFeeds, schreiben: darfPersoenlich }] as gruppe (gruppe.titel)}
        {#if gruppe.liste.length > 0 || (gruppe.schreiben && gruppe.titel === t('Nur für mich'))}
          <section>
            <h3 class="mb-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">{gruppe.titel}</h3>
            {#if gruppe.liste.length === 0}
              <p class="text-sm text-muted-foreground">{t('Noch nichts eingetragen.')}</p>
            {:else}
              <ul class="space-y-1.5">
                {#each gruppe.liste as f (f.id)}
                  <li class="rounded-xl bg-muted/40 px-3 py-2">
                    <div class="flex items-center gap-2">
                      <div class="min-w-0 flex-1">
                        <p class="truncate text-sm font-medium">{f.title}</p>
                        <p class="truncate text-xs text-muted-foreground">{host(f.url)} · {f.count === 1 ? t('1 Meldung') : t('{0} Meldungen', [f.count])}</p>
                      </div>
                      {#if f.editable}
                        <button class="touch-target text-muted-foreground hover:text-destructive" onclick={() => entfernen(f)} aria-label={t('„{0}“ entfernen', [f.title])}>
                          <Trash2 class="h-4 w-4" />
                        </button>
                      {/if}
                    </div>
                    {#if f.last_error}
                      <p class="mt-1 flex items-start gap-1.5 text-xs text-amber-700 dark:text-amber-400">
                        <TriangleAlert class="mt-0.5 h-3.5 w-3.5 shrink-0" /> {f.last_error}
                      </p>
                    {/if}
                  </li>
                {/each}
              </ul>
            {/if}
          </section>
        {/if}
      {/each}

      {#if darfFamilie || darfPersoenlich}
        <form class="space-y-2.5 border-t border-border pt-4" onsubmit={hinzufuegen}>
          <h3 class="text-sm font-semibold">{t('Quelle hinzufügen')}</h3>
          <input class="input" type="url" inputmode="url" placeholder={t('Adresse des Feeds oder der Webseite')} bind:value={adresse} aria-label={t('Adresse')} />
          <input class="input" placeholder={t('Name (optional)')} bind:value={titel} maxlength="80" aria-label={t('Name')} />
          {#if darfFamilie && darfPersoenlich}
            <div class="flex gap-1.5">
              <button type="button" class="chip" aria-pressed={familie} onclick={() => (familie = true)}>{t('Für die Familie')}</button>
              <button type="button" class="chip" aria-pressed={!familie} onclick={() => (familie = false)}>{t('Nur für mich')}</button>
            </div>
          {/if}
          {#if formFehler}
            <p class="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{formFehler}</p>
          {/if}
          <button class="btn-primary w-full" disabled={!adresse.trim() || sendet}>
            <Plus class="h-4 w-4" /> {sendet ? t('Wird geprüft…') : t('Hinzufügen')}
          </button>
        </form>
      {/if}
    </div>
  </Modal>
{/if}
