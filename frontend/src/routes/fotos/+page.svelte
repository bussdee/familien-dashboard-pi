<script lang="ts">
  import { t } from '$lib/i18n';
  import { onMount } from 'svelte';
  import { ChevronLeft, ChevronRight, Expand, Images, MonitorPlay, Trash2, Upload, X } from 'lucide-svelte';
  import { ApiError, photosApi } from '$lib/api';
  import { darfEintragen, session } from '$lib/stores';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import type { Photo } from '$lib/types';

  /**
   * Das Fotoarchiv. Bis 2.4 war das eine Kachel auf der Übersicht; jetzt sind
   * die Fotos der Bildschirmschoner (siehe Verwaltung → Bildschirm) und liegen
   * hier: ansehen, hochladen, löschen, die Diashow von Hand starten.
   *
   * Hochladen und Löschen ist Elternsache — am Wandgerät mit der PIN eines
   * Elternteils.
   */
  const ACCEPT = 'image/jpeg,image/png,image/webp,image/gif,image/avif';

  let photos = $state<Photo[]>([]);
  let laden = $state(true);
  let fehler = $state('');
  let hochladen = $state(false);
  let fortschritt = $state(0);
  let ziehen = $state(false);
  let dateiwahl: HTMLInputElement | null = $state(null);

  let gross = $state<number | null>(null);
  const aktuell = $derived(gross !== null ? (photos[gross] ?? null) : null);

  const modus = $derived(einstellungen.werte?.['schoner.modus'] ?? 'wandgeraet');
  const minutenBis = $derived(einstellungen.werte?.['schoner.leerlauf_min'] ?? '5');
  const admin = $derived($session.user?.role === 'admin');

  async function ladeFotos() {
    try {
      photos = (await photosApi.list()).photos;
      fehler = '';
    } catch (e) {
      fehler = e instanceof ApiError ? e.message : t('Die Fotos konnten nicht geladen werden.');
    } finally {
      laden = false;
    }
  }

  async function hochladenStarten(dateien: FileList | File[] | null) {
    const liste = [...(dateien ?? [])];
    if (liste.length === 0 || hochladen) return;
    hochladen = true;
    fortschritt = 0;
    try {
      const r = await photosApi.upload(liste, (p) => (fortschritt = p));
      const uebersprungen = Object.entries(r.skipped ?? {});
      toast(r.count === 1 ? t('1 Foto hinzugefügt') : t('{0} Fotos hinzugefügt', [r.count]), { ton: 'erfolg' });
      if (uebersprungen.length > 0) {
        toast(uebersprungen.map(([n, grund]) => `${n}: ${grund}`).join(' · '), { ton: 'fehler', dauer: 7000 });
      }
      await ladeFotos();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Upload fehlgeschlagen'), { ton: 'fehler' });
    } finally {
      hochladen = false;
      fortschritt = 0;
      if (dateiwahl) dateiwahl.value = '';
    }
  }

  async function loeschen(p: Photo) {
    const ok = await confirmAction({
      title: t('Foto löschen?'),
      message: t('„{0}“ wird endgültig entfernt.', [p.name]),
    });
    if (!ok) return;
    try {
      await photosApi.remove(p.name);
      gross = null;
      await ladeFotos();
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Löschen fehlgeschlagen'), { ton: 'fehler' });
    }
  }

  function blaettern(schritt: number) {
    if (gross === null || photos.length === 0) return;
    gross = (gross + schritt + photos.length) % photos.length;
  }

  function ablegen(event: DragEvent) {
    event.preventDefault();
    ziehen = false;
    if ($darfEintragen) void hochladenStarten(event.dataTransfer?.files ?? null);
  }

  onMount(() => void ladeFotos());
</script>

<svelte:head><title>{t('Fotos · Familien Dashboard')}</title></svelte:head>

<svelte:window
  onkeydown={(e) => {
    if (gross === null) return;
    if (e.key === 'Escape') gross = null;
    if (e.key === 'ArrowRight') blaettern(1);
    if (e.key === 'ArrowLeft') blaettern(-1);
  }}
/>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="mx-auto w-full max-w-5xl px-4 py-5 sm:py-7"
  ondragover={(e) => {
    if ($darfEintragen) {
      e.preventDefault();
      ziehen = true;
    }
  }}
  ondragleave={() => (ziehen = false)}
  ondrop={ablegen}
>
  <header class="mb-5 flex flex-wrap items-end justify-between gap-3">
    <div>
      <p class="text-[11px] font-medium uppercase tracking-[0.2em] text-muted-foreground">
        {photos.length === 1 ? t('1 Foto') : t('{0} Fotos', [photos.length])}
      </p>
      <h1 class="seiten-titel mt-1">{t('Fotos')}</h1>
    </div>
    <div class="flex items-center gap-2">
      {#if photos.length > 0}
        <a href="/diashow" class="btn-outline text-sm"><Expand class="h-4 w-4" /> {t('Diashow starten')}</a>
      {/if}
      {#if $darfEintragen}
        <input
          bind:this={dateiwahl}
          type="file"
          accept={ACCEPT}
          multiple
          class="hidden"
          onchange={(e) => hochladenStarten((e.currentTarget as HTMLInputElement).files)}
        />
        <button class="btn-primary text-sm" onclick={() => dateiwahl?.click()} disabled={hochladen}>
          <Upload class="h-4 w-4" /> {t('Fotos hochladen')}
        </button>
      {/if}
    </div>
  </header>

  <!-- Wann und wie der Bildschirmschoner startet: Das ist der Zweck der Fotos,
       und wer hier ist, fragt sich genau das. -->
  <p class="mb-4 flex items-start gap-3 rounded-2xl bg-muted/50 p-4 text-sm text-muted-foreground">
    <MonitorPlay class="mt-0.5 h-5 w-5 shrink-0" />
    <span>
      {#if modus === 'aus'}
        {t('Der Bildschirmschoner ist ausgeschaltet.')}
      {:else if modus === 'alle'}
        {t('Nach {0} Minuten ohne Berührung zeigt jedes Gerät die Diashow.', [minutenBis])}
      {:else}
        {t('Nach {0} Minuten ohne Berührung zeigt das Wandgerät die Diashow.', [minutenBis])}
      {/if}
      {#if admin}
        <a href="/admin?bereich=bildschirm" class="text-primary hover:underline">{t('Einstellen →')}</a>
      {/if}
    </span>
  </p>

  {#if fehler}
    <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-3 text-sm text-destructive">{fehler}</p>
  {/if}

  {#if hochladen}
    <div class="mb-4 rounded-xl bg-primary/10 px-4 py-3 text-sm text-primary">
      <p class="mb-1.5">{t('{0}% übertragen', [fortschritt])}</p>
      <div class="h-2 overflow-hidden rounded-full bg-primary/20">
        <div class="h-full bg-primary transition-all" style="width: {fortschritt}%"></div>
      </div>
    </div>
  {/if}

  {#if ziehen}
    <div class="mb-4 flex items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-primary bg-primary/10 py-8 text-primary">
      <Upload class="h-6 w-6" /> {t('Fotos hier ablegen')}
    </div>
  {/if}

  {#if !laden && photos.length === 0 && !fehler}
    <div class="card flex flex-col items-center gap-2 p-10 text-center">
      <Images class="h-10 w-10 text-muted-foreground opacity-40" />
      <p class="font-medium">{t('Noch keine Fotos')}</p>
      <p class="max-w-sm text-sm text-muted-foreground">
        {$darfEintragen
          ? t('Lade Fotos hoch, oder ziehe sie einfach hierher. JPG · PNG · WEBP · GIF · AVIF, bis 25 MB je Bild.')
          : t('Die Eltern laden die Fotos hoch.')}
      </p>
      {#if $darfEintragen}
        <button class="btn-primary mt-2" onclick={() => dateiwahl?.click()}><Upload class="h-4 w-4" /> {t('Fotos hochladen')}</button>
      {/if}
    </div>
  {:else}
    <ul class="grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-4 {laden ? 'opacity-60' : ''}">
      {#each photos as p, i (p.name)}
        <li>
          <button class="group relative block aspect-square w-full overflow-hidden rounded-xl bg-muted/40" onclick={() => (gross = i)} aria-label={p.name}>
            <img src={photosApi.url(p.name)} alt="" loading="lazy" class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" />
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

{#if aktuell}
  <!-- svelte-ignore a11y_no_static_element_interactions, a11y_click_events_have_key_events, a11y_interactive_supports_focus -->
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/90"
    role="dialog"
    aria-modal="true"
    aria-label={aktuell.name}
    onclick={(e) => e.target === e.currentTarget && (gross = null)}
  >
    <img src={photosApi.url(aktuell.name)} alt={aktuell.name} class="max-h-full max-w-full object-contain" />
    <button class="absolute right-3 top-3 rounded-full bg-white/15 p-3 text-white backdrop-blur hover:bg-white/25" onclick={() => (gross = null)} aria-label={t('Schließen')}>
      <X class="h-5 w-5" />
    </button>
    {#if photos.length > 1}
      <button class="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-white/15 p-3 text-white backdrop-blur hover:bg-white/25" onclick={() => blaettern(-1)} aria-label={t('Vorheriges Foto')}>
        <ChevronLeft class="h-6 w-6" />
      </button>
      <button class="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-white/15 p-3 text-white backdrop-blur hover:bg-white/25" onclick={() => blaettern(1)} aria-label={t('Nächstes Foto')}>
        <ChevronRight class="h-6 w-6" />
      </button>
    {/if}
    {#if $darfEintragen}
      <button class="absolute bottom-4 right-4 flex items-center gap-2 rounded-full bg-white/15 px-4 py-2.5 text-sm text-white backdrop-blur hover:bg-destructive" onclick={() => loeschen(aktuell)}>
        <Trash2 class="h-4 w-4" /> {t('Löschen')}
      </button>
    {/if}
  </div>
{/if}
