<script lang="ts">
  import { t } from '$lib/i18n';
  import { LayoutGrid } from 'lucide-svelte';
  import { ApiError, einstellungenApi } from '$lib/api';
  import { einstellungen } from '$lib/stores/einstellungen.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import type { Einstellungen } from '$lib/types';
  import Schalter from './Schalter.svelte';

  /**
   * Welche Bereiche es in dieser Familie gibt. Was hier ausgeschaltet ist,
   * verschwindet aus dem Menü und aus der Übersicht — und der Server lehnt es
   * auch ab, nicht nur die Oberfläche.
   *
   * Die Neuigkeiten sind standardmäßig aus: Sie rufen fremde Adressen ab, und
   * das soll nie ohne Absicht geschehen.
   */
  let arbeitet = $state(false);

  const werte = $derived(einstellungen.werte);
  const an = (k: keyof Einstellungen) => werte?.[k] === '1';

  async function grenze(wert: string) {
    if (arbeitet) return;
    arbeitet = true;
    try {
      einstellungen.uebernehmen(await einstellungenApi.put({ 'bonus.max': wert }));
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      arbeitet = false;
    }
  }

  async function setzen(k: keyof Einstellungen, neu: boolean) {
    if (arbeitet) return;
    arbeitet = true;
    try {
      einstellungen.uebernehmen(await einstellungenApi.put({ [k]: neu ? '1' : '0' }));
    } catch (e) {
      toast(e instanceof ApiError ? e.message : t('Konnte nicht speichern'), { ton: 'fehler' });
    } finally {
      arbeitet = false;
    }
  }
</script>

<section class="card mb-4 p-5">
  <h2 class="mb-1 flex items-center gap-2 font-semibold">
    <LayoutGrid class="h-5 w-5" /> {t('Bereiche')}
  </h2>
  <p class="mb-2 text-xs text-muted-foreground">
    {t('Was ausgeschaltet ist, verschwindet für alle aus dem Menü.')}
  </p>

  <div class="divide-y divide-border">
    <Schalter
      an={an('modul.rezepte')}
      gesperrt={arbeitet || !werte}
      titel={t('Rezeptbuch')}
      hinweis={t('Rezepte per Link oder von Hand, mit Zutaten für die Einkaufsliste und den Essensplan.')}
      onUmschalten={(neu) => setzen('modul.rezepte', neu)}
    />
    <Schalter
      an={an('modul.neuigkeiten')}
      gesperrt={arbeitet || !werte}
      titel={t('Neuigkeiten (Feeds)')}
      hinweis={t('Zeitungen, Blogs und die Schule als Feed. Der Server ruft die Adressen regelmäßig ab; ausgeschaltet ruft er nichts ab.')}
      onUmschalten={(neu) => setzen('modul.neuigkeiten', neu)}
    />
    <Schalter
      an={an('modul.tagesbonus')}
      gesperrt={arbeitet || !werte}
      titel={t('Tagesbonus für Kinder')}
      hinweis={t('Wer das Dashboard jeden Tag öffnet, bekommt Punkte: am ersten Tag 1, am zweiten Tag in Folge 2, am dritten 3 … Fehlt ein Tag, geht es wieder bei 1 los. Nur für Kinder, persönlich angemeldet.')}
      onUmschalten={(neu) => setzen('modul.tagesbonus', neu)}
    >
      {#if an('modul.tagesbonus')}
        <div class="mt-2">
          <p class="mb-1 text-xs text-muted-foreground">{t('Höchstens pro Tag')}</p>
          <div class="flex flex-wrap gap-1.5">
            {#each ['0', '5', '10', '20', '30'] as m (m)}
              <button
                type="button"
                class="chip"
                aria-pressed={werte?.['bonus.max'] === m}
                disabled={arbeitet}
                onclick={() => grenze(m)}
              >
                {m === '0' ? t('Ohne Grenze') : t('{0} Punkte', [m])}
              </button>
            {/each}
          </div>
        </div>
      {/if}
    </Schalter>
    {#if an('modul.neuigkeiten')}
      <Schalter
        an={an('modul.neuigkeiten_persoenlich')}
        gesperrt={arbeitet || !werte}
        titel={t('Persönliche Listen erlauben')}
        hinweis={t('Jede Person darf eigene Feeds eintragen, die nur sie sieht. Familien-Feeds pflegen die Eltern. Jede Person kann die Neuigkeiten außerdem für sich ausblenden.')}
        onUmschalten={(neu) => setzen('modul.neuigkeiten_persoenlich', neu)}
      />
    {/if}
  </div>
</section>
