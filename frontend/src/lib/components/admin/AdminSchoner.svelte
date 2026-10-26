<script lang="ts">
  import { t } from '$lib/i18n';
  import { Images, MonitorPlay } from 'lucide-svelte';
  import { ApiError, einstellungenApi } from '$lib/api';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import type { Einstellungen } from '$lib/types';

  /**
   * Der Bildschirmschoner: wann er startet, was er zeigt und wie dunkel er
   * nachts wird. Die Fotos selbst liegen unter „Fotos" im Menü.
   */
  let arbeitet = $state(false);
  const w = $derived(einstellungen.werte);

  async function setzen(daten: Partial<Einstellungen>) {
    if (arbeitet) return;
    arbeitet = true;
    try {
      einstellungen.uebernehmen(await einstellungenApi.put(daten));
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      arbeitet = false;
    }
  }

  const MODI: { wert: Einstellungen['schoner.modus']; label: string; hinweis: string }[] = [
    { wert: 'wandgeraet', label: t('Nur das Wandgerät'), hinweis: t('Handys und Rechner bleiben, wie sie sind.') },
    { wert: 'alle', label: t('Jedes Gerät'), hinweis: t('Auch Handys und Tablets mit einer Person darauf.') },
    { wert: 'aus', label: t('Aus'), hinweis: t('Nichts startet von selbst. Die Diashow gibt es weiter von Hand.') },
  ];

  const MINUTEN = [2, 5, 10, 20, 30, 60];
</script>

<section class="card mb-4 p-5">
  <h2 class="mb-1 flex items-center gap-2 font-semibold">
    <MonitorPlay class="h-5 w-5" /> {t('Bildschirmschoner')}
  </h2>
  <p class="mb-4 text-xs text-muted-foreground">
    {t('Nach einer Weile ohne Berührung zeigt der Bildschirm die Diashow. Eine Berührung bringt die Seite zurück, auf der man war.')}
  </p>

  {#if w}
    <fieldset class="mb-4">
      <legend class="mb-1.5 text-sm font-medium">{t('Wo?')}</legend>
      <div class="space-y-1.5">
        {#each MODI as m (m.wert)}
          <label class="flex cursor-pointer items-start gap-3 rounded-xl border border-border px-3 py-2.5 has-[:checked]:border-primary has-[:checked]:bg-primary/5">
            <input
              type="radio"
              name="schoner-modus"
              class="mt-1 accent-[hsl(var(--primary))]"
              checked={w['schoner.modus'] === m.wert}
              disabled={arbeitet}
              onchange={() => setzen({ 'schoner.modus': m.wert })}
            />
            <span>
              <span class="block text-sm font-medium">{m.label}</span>
              <span class="block text-xs text-muted-foreground">{m.hinweis}</span>
            </span>
          </label>
        {/each}
      </div>
    </fieldset>

    {#if w['schoner.modus'] !== 'aus'}
      <div class="mb-4">
        <p class="mb-1.5 text-sm font-medium">{t('Startet nach')}</p>
        <div class="flex flex-wrap gap-1.5">
          {#each MINUTEN as m (m)}
            <button
              class="chip"
              aria-pressed={w['schoner.leerlauf_min'] === String(m)}
              disabled={arbeitet}
              onclick={() => setzen({ 'schoner.leerlauf_min': String(m) })}
            >
              {m === 60 ? t('1 Stunde') : t('{0} Min.', [m])}
            </button>
          {/each}
        </div>
      </div>

      <div class="mb-4">
        <p class="mb-1.5 text-sm font-medium">{t('Zeigt')}</p>
        <div class="flex flex-wrap gap-1.5">
          <button class="chip" aria-pressed={w['schoner.inhalt'] === 'diashow'} disabled={arbeitet} onclick={() => setzen({ 'schoner.inhalt': 'diashow' })}>
            {t('Fotos mit Uhr und Wetter')}
          </button>
          <button class="chip" aria-pressed={w['schoner.inhalt'] === 'uhr'} disabled={arbeitet} onclick={() => setzen({ 'schoner.inhalt': 'uhr' })}>
            {t('Nur Uhr und Wetter')}
          </button>
        </div>
        <p class="mt-1.5 text-xs text-muted-foreground">
          {t('Ohne Fotos zeigt der Bildschirmschoner immer nur Uhr und Wetter.')}
        </p>
      </div>

      <div>
        <p class="mb-1.5 text-sm font-medium">{t('Nachts dunkler')}</p>
        <div class="grid grid-cols-2 gap-2">
          <label class="block text-xs text-muted-foreground">
            {t('Von')}
            <input
              class="input mt-1"
              type="time"
              value={w['schoner.nacht_von']}
              disabled={arbeitet}
              onchange={(e) => (e.currentTarget as HTMLInputElement).value && setzen({ 'schoner.nacht_von': (e.currentTarget as HTMLInputElement).value })}
            />
          </label>
          <label class="block text-xs text-muted-foreground">
            {t('Bis')}
            <input
              class="input mt-1"
              type="time"
              value={w['schoner.nacht_bis']}
              disabled={arbeitet}
              onchange={(e) => (e.currentTarget as HTMLInputElement).value && setzen({ 'schoner.nacht_bis': (e.currentTarget as HTMLInputElement).value })}
            />
          </label>
        </div>
        <label class="mt-3 block text-xs text-muted-foreground">
          {t('Wie dunkel: {0} %', [w['schoner.nacht_dunkel']])}
          <input
            class="mt-1 w-full accent-[hsl(var(--primary))]"
            type="range"
            min="0"
            max="90"
            step="10"
            value={w['schoner.nacht_dunkel']}
            disabled={arbeitet}
            onchange={(e) => setzen({ 'schoner.nacht_dunkel': (e.currentTarget as HTMLInputElement).value })}
          />
        </label>
        <p class="mt-1 text-xs text-muted-foreground">{t('0 % lässt den Bildschirm nachts so hell wie tagsüber.')}</p>
      </div>
    {/if}
  {:else}
    <p class="text-sm text-muted-foreground">{t('Lade…')}</p>
  {/if}

  <a href="/fotos" class="mt-5 flex items-center gap-2 rounded-xl border border-border px-3 py-2.5 text-sm hover:bg-accent">
    <Images class="h-4 w-4 text-muted-foreground" />
    <span class="flex-1">{t('Fotos hochladen und verwalten')}</span>
    <span class="text-muted-foreground">→</span>
  </a>
</section>
