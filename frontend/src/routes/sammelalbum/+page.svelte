<script lang="ts">
  import { t, tServer, dfLocale } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { addMonths, eachDayOfInterval, endOfMonth, format, getISODay, isToday, startOfMonth } from 'date-fns';
  import { Award, ChevronLeft, ChevronRight, Flame, ShieldCheck } from 'lucide-svelte';
  import { albumApi, bonusApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { tagesbonus } from '$lib/stores/tagesbonus.svelte';
  import type { AlbumEintrag, BonusKalender } from '$lib/types';

  /**
   * Das Sammelalbum: alle Abzeichen — die, die man hat, und die, die noch
   * fehlen, als Umriss. Geheime stehen als Fragezeichen da, bis man sie hat.
   * Darüber, für Kinder mit Tagesbonus, die Serie: wie lange sie hält, was der
   * nächste Meilenstein ist, und ein Kalender mit einer Flamme für jeden Tag.
   */
  let album = $state<AlbumEintrag[]>([]);
  let kalender = $state<BonusKalender | null>(null);
  let monat = $state(startOfMonth(new Date()));
  let laden = $state(true);

  const mitSerie = $derived(!!tagesbonus.stand?.eligible);
  const serie = $derived(tagesbonus.stand?.streak ?? 0);
  const naechster = $derived(tagesbonus.stand?.next_milestone ?? 0);
  const bisDahin = $derived(tagesbonus.stand?.days_to_milestone ?? 0);
  const jokerAn = $derived(einstellungen.werte?.['bonus.joker'] === '1');
  const gesammelt = $derived(album.filter((a) => a.earned).length);

  const tage = $derived(eachDayOfInterval({ start: monat, end: endOfMonth(monat) }));
  const versatz = $derived(getISODay(monat) - 1); // Montag zuerst
  const eintrag = (d: Date) => kalender?.days.find((k) => k.day === format(d, 'yyyy-MM-dd')) ?? null;
  const istAktuellerMonat = $derived(format(monat, 'yyyy-MM') === format(new Date(), 'yyyy-MM'));
  const WOCHENTAGE = Array.from({ length: 7 }, (_, i) => format(new Date(2024, 0, 1 + i), 'EEEEEE', { locale: dfLocale }));

  async function ladeKalender() {
    if (!mitSerie) return;
    try {
      kalender = await bonusApi.kalender(format(monat, 'yyyy-MM'));
    } catch {
      kalender = null;
    }
  }

  function blaettern(schritt: number) {
    monat = addMonths(monat, schritt);
    void ladeKalender();
  }

  $effect(() => {
    if (mitSerie && !kalender) void ladeKalender();
  });

  onMount(async () => {
    try {
      album = await albumApi.liste();
    } catch {
      album = [];
    } finally {
      laden = false;
    }
  });
</script>

<svelte:head><title>{t('Sammelalbum · Familien Dashboard')}</title></svelte:head>

<div class="mx-auto w-full max-w-4xl px-4 py-5 sm:py-7">
  <header class="mb-6">
    <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
      {t('{0} von {1} Abzeichen', [gesammelt, album.length])}
    </p>
    <h1 class="seiten-titel mt-1">{t('Sammelalbum')}</h1>
  </header>

  {#if $session.device}
    <p class="text-sm text-muted-foreground">{t('Das Sammelalbum gehört einer Person. Bitte anmelden.')}</p>
  {/if}

  {#if mitSerie}
    <!-- ------------------------------------------------------ Die Serie -->
    <section class="card mb-6 p-5">
      <div class="flex flex-wrap items-center gap-x-8 gap-y-4">
        <div class="flex items-center gap-3">
          <Flame class="h-10 w-10 text-amber-500" />
          <div>
            <p class="font-display text-5xl font-medium leading-none tabular-nums">{serie}</p>
            <p class="text-xs text-muted-foreground">{serie === 1 ? t('Tag in Folge') : t('Tage in Folge')}</p>
          </div>
        </div>
        {#if kalender}
          <div>
            <p class="text-2xl font-semibold tabular-nums">{kalender.best}</p>
            <p class="text-xs text-muted-foreground">{t('längste Serie')}</p>
          </div>
          <div>
            <p class="text-2xl font-semibold tabular-nums">{kalender.total_days}</p>
            <p class="text-xs text-muted-foreground">{t('Tage insgesamt')}</p>
          </div>
        {/if}
        {#if jokerAn}
          <p class="flex items-center gap-1.5 text-sm {tagesbonus.stand?.joker_available ? 'text-primary' : 'text-muted-foreground'}">
            <ShieldCheck class="h-4 w-4" />
            {tagesbonus.stand?.joker_available ? t('Serienschutz bereit') : t('Serienschutz diese Woche verbraucht')}
          </p>
        {/if}
      </div>

      {#if naechster > 0}
        <div class="mt-5">
          <div class="mb-1 flex justify-between text-xs text-muted-foreground">
            <span>{bisDahin === 1 ? t('Morgen: 🏅 {0} Tage!', [naechster]) : t('Noch {0} Tage bis 🏅 {1} Tage', [bisDahin, naechster])}</span>
            <span class="tabular-nums">{serie}/{naechster}</span>
          </div>
          <div class="h-2 overflow-hidden rounded-full bg-muted">
            <div class="h-full rounded-full bg-gradient-to-r from-amber-400 to-primary transition-[width] duration-700" style="width: {Math.min(100, (serie / naechster) * 100)}%"></div>
          </div>
        </div>
      {/if}

      {#if kalender}
        <div class="mt-4 flex flex-wrap gap-1.5">
          {#each kalender.milestones as m (m)}
            <span
              class="rounded-full border px-2.5 py-0.5 text-xs tabular-nums
                {kalender.best >= m ? 'border-amber-500/50 bg-amber-500/15 text-amber-700 dark:text-amber-300' : 'border-[color:var(--haarlinie-stark)] text-muted-foreground'}"
              title={kalender.best >= m ? t('Geschafft') : t('Noch nicht geschafft')}
            >
              🏅 {m}
            </span>
          {/each}
        </div>
      {/if}

      <!-- Der Kalender: eine Flamme für jeden Tag, ein Schild für einen
           geretteten. Lücken sieht man auf einen Blick. -->
      <div class="mx-auto mt-6 max-w-md">
        <div class="mb-2 flex items-center justify-between">
          <button class="btn-ghost rounded-full px-2" onclick={() => blaettern(-1)} aria-label={t('Monat zurück')}>
            <ChevronLeft class="h-5 w-5" />
          </button>
          <p class="text-sm font-medium">{format(monat, 'LLLL yyyy', { locale: dfLocale })}</p>
          <button class="btn-ghost rounded-full px-2" onclick={() => blaettern(1)} disabled={istAktuellerMonat} aria-label={t('Monat vor')}>
            <ChevronRight class="h-5 w-5" />
          </button>
        </div>
        <div class="grid grid-cols-7 gap-1 text-center">
          {#each WOCHENTAGE as w (w)}
            <span class="pb-1 text-[11px] uppercase text-muted-foreground">{w}</span>
          {/each}
          {#each Array(versatz) as _, i (i)}
            <span></span>
          {/each}
          {#each tage as d (d.getTime())}
            {@const e = eintrag(d)}
            <span
              class="flex aspect-square flex-col items-center justify-center rounded-xl text-xs
                {e && !e.joker ? 'bg-amber-500/15' : e?.joker ? 'bg-primary/10' : 'bg-muted/30'}
                {isToday(d) ? 'ring-2 ring-primary' : ''}"
              title={e ? (e.joker ? t('Gerettet vom Serienschutz') : t('Tag {0} der Serie · +{1}', [e.streak, e.points])) : ''}
            >
              <span class="text-muted-foreground">{format(d, 'd')}</span>
              <span class="text-base leading-none" aria-hidden="true">{e ? (e.joker ? '🛡️' : '🔥') : ''}</span>
            </span>
          {/each}
        </div>
      </div>
    </section>
  {/if}

  <!-- ------------------------------------------------------- Das Album -->
  <h2 class="mb-3 flex items-center gap-2 text-lg font-semibold">
    <Award class="h-5 w-5" /> {t('Abzeichen')}
  </h2>
  {#if !laden && album.length === 0}
    <p class="text-sm text-muted-foreground">{t('Das Album konnte nicht geladen werden.')}</p>
  {/if}
  <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
    {#each album as a (a.id)}
      <li
        class="card flex flex-col items-center gap-1.5 p-4 text-center
          {a.earned ? '' : 'opacity-60'}"
      >
        {#if a.secret && !a.earned}
          <span class="flex h-14 w-14 items-center justify-center rounded-full border-2 border-dashed border-muted-foreground/40 text-2xl text-muted-foreground">?</span>
          <p class="text-sm font-semibold">{t('Geheim')}</p>
          <p class="text-xs text-muted-foreground">{t('Wird verraten, sobald du es hast.')}</p>
        {:else}
          <span
            class="flex h-14 w-14 items-center justify-center rounded-full text-3xl
              {a.earned ? 'bg-amber-500/15' : 'bg-muted grayscale'}"
          >
            {a.emoji}
          </span>
          <p class="text-sm font-semibold leading-tight">{tServer(a.label)}</p>
          <p class="text-xs text-muted-foreground">{tServer(a.description)}</p>
          {#if a.earned}<p class="text-[11px] font-medium text-amber-600 dark:text-amber-400">{t('Gesammelt')}</p>{/if}
        {/if}
      </li>
    {/each}
  </ul>
</div>
