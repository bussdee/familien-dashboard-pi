<script lang="ts">
  import { t } from '$lib/i18n';
  import type { Rezept, RezeptEntwurf } from '$lib/types';

  /**
   * Das Formular fürs Rezeptbuch: zum Eintragen von Hand und zum Ändern eines
   * Rezepts, das per Link kam. Zutaten und Schritte sind je eine Zeile ein
   * Eintrag — so tippt man sie ein, und so wandern sie später auf die Liste.
   */
  let {
    rezept = null,
    speichert = false,
    onSpeichern,
    onAbbrechen,
  }: {
    rezept?: Rezept | null;
    speichert?: boolean;
    onSpeichern: (daten: RezeptEntwurf) => void | Promise<void>;
    onAbbrechen: () => void;
  } = $props();

  // svelte-ignore state_referenced_locally
  let titel = $state(rezept?.title ?? '');
  // svelte-ignore state_referenced_locally
  let portionen = $state(rezept?.servings ?? '');
  // svelte-ignore state_referenced_locally
  let vorbereiten = $state(rezept?.prep_min ? String(rezept.prep_min) : '');
  // svelte-ignore state_referenced_locally
  let kochen = $state(rezept?.cook_min ? String(rezept.cook_min) : '');
  // svelte-ignore state_referenced_locally
  let kategorie = $state(rezept?.category ?? '');
  // svelte-ignore state_referenced_locally
  let zutaten = $state((rezept?.ingredients ?? []).join('\n'));
  // svelte-ignore state_referenced_locally
  let schritte = $state((rezept?.steps ?? []).join('\n'));
  // svelte-ignore state_referenced_locally
  let notiz = $state(rezept?.note ?? '');
  // svelte-ignore state_referenced_locally
  let quelle = $state(rezept?.source_url ?? '');

  const zahl = (s: string) => Math.max(0, Number.parseInt(s.replace(/\D/g, ''), 10) || 0);
  const zeilen = (s: string) => s.split('\n').map((z) => z.trim()).filter(Boolean);

  function absenden(event: SubmitEvent) {
    event.preventDefault();
    if (!titel.trim() || speichert) return;
    const v = zahl(vorbereiten);
    const k = zahl(kochen);
    void onSpeichern({
      title: titel.trim(),
      servings: portionen.trim(),
      prep_min: v,
      cook_min: k,
      total_min: v + k,
      category: kategorie.trim(),
      note: notiz.trim(),
      source_url: quelle.trim(),
      ingredients: zeilen(zutaten),
      steps: zeilen(schritte),
    });
  }
</script>

<form class="space-y-3" onsubmit={absenden}>
  <input class="input" placeholder={t('Name des Rezepts')} bind:value={titel} maxlength="200" aria-label={t('Name des Rezepts')} />

  <div class="grid grid-cols-3 gap-2">
    <label class="block text-xs text-muted-foreground">
      {t('Portionen')}
      <input class="input mt-1" bind:value={portionen} maxlength="60" placeholder={t('z. B. 4')} />
    </label>
    <label class="block text-xs text-muted-foreground">
      {t('Vorbereiten (Min.)')}
      <input class="input mt-1" inputmode="numeric" bind:value={vorbereiten} maxlength="4" />
    </label>
    <label class="block text-xs text-muted-foreground">
      {t('Kochen (Min.)')}
      <input class="input mt-1" inputmode="numeric" bind:value={kochen} maxlength="4" />
    </label>
  </div>

  <label class="block text-xs text-muted-foreground">
    {t('Kategorie (optional)')}
    <input class="input mt-1" bind:value={kategorie} maxlength="60" placeholder={t('z. B. Hauptgericht, Kuchen')} />
  </label>

  <label class="block text-xs text-muted-foreground">
    {t('Zutaten — eine pro Zeile')}
    <textarea class="input mt-1 min-h-[7rem]" bind:value={zutaten} placeholder={'500 g Spaghetti\n2 Dosen Tomaten'}></textarea>
  </label>

  <label class="block text-xs text-muted-foreground">
    {t('Zubereitung — ein Schritt pro Zeile')}
    <textarea class="input mt-1 min-h-[7rem]" bind:value={schritte}></textarea>
  </label>

  <label class="block text-xs text-muted-foreground">
    {t('Notiz (optional)')}
    <input class="input mt-1" bind:value={notiz} maxlength="1000" />
  </label>

  <label class="block text-xs text-muted-foreground">
    {t('Quelle (optional)')}
    <input class="input mt-1" type="url" bind:value={quelle} placeholder="https://…" />
  </label>

  <div class="flex gap-2 pt-1">
    <button type="button" class="btn-outline flex-1" onclick={onAbbrechen}>{t('Abbrechen')}</button>
    <button class="btn-primary flex-1" disabled={!titel.trim() || speichert}>
      {speichert ? t('Einen Moment…') : t('Speichern')}
    </button>
  </div>
</form>
