<script lang="ts">
  import { onMount } from 'svelte';
  import { Download, FileText, HardDrive, TriangleAlert, Trash2, Upload, X } from 'lucide-svelte';
  import { ApiError, filesApi } from '$lib/api';
  import { session } from '$lib/stores';
  import type { FileListing, StoredFile } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  let listing = $state<FileListing | null>(null);
  let loading = $state(true);

  let uploading = $state(false);
  let progress = $state(0);
  let message = $state('');
  let error = $state('');
  let dragging = $state(false);
  let fileInput: HTMLInputElement | null = $state(null);

  // Hochladen und Löschen gehören dem Administrator, Herunterladen allen.
  // Am Wandgerät ist niemand angemeldet, also ist dort auch niemand Admin —
  // die Familie kann dort trotzdem herunterladen.
  const istAdmin = $derived($session.user?.role === 'admin');
  const dateien = $derived(listing?.files ?? []);

  function groesse(bytes: number): string {
    if (bytes >= 1_073_741_824) return `${(bytes / 1_073_741_824).toFixed(1)} GB`;
    if (bytes >= 1_048_576) return `${(bytes / 1_048_576).toFixed(1)} MB`;
    if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${bytes} B`;
  }

  const datum = (iso: string) =>
    new Date(iso).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: '2-digit' });

  async function load() {
    try {
      listing = await filesApi.list();
    } catch {
      listing = null;
    } finally {
      loading = false;
    }
  }

  async function upload(auswahl: FileList | File[] | null) {
    const liste = [...(auswahl ?? [])];
    if (liste.length === 0 || uploading) return;

    uploading = true;
    progress = 0;
    error = '';
    message = '';

    try {
      const ergebnis = await filesApi.upload(liste, (prozent) => (progress = prozent));
      const uebersprungen = Object.entries(ergebnis.skipped ?? {});
      message =
        ergebnis.count === 1 ? '1 Datei hinzugefügt' : `${ergebnis.count} Dateien hinzugefügt`;
      if (uebersprungen.length > 0) {
        error = uebersprungen.map(([name, grund]) => `${name}: ${grund}`).join(' · ');
      }
      await load();
      setTimeout(() => (message = ''), 4000);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Upload fehlgeschlagen';
    } finally {
      uploading = false;
      progress = 0;
      if (fileInput) fileInput.value = '';
    }
  }

  async function entfernen(datei: StoredFile) {
    const ok = await confirmAction({
      title: 'Datei löschen?',
      message: `„${datei.name}“ wird endgültig entfernt.`,
    });
    if (!ok) return;
    try {
      await filesApi.remove(datei.name);
      await load();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Löschen fehlgeschlagen';
    }
  }

  function onDrop(event: DragEvent) {
    event.preventDefault();
    dragging = false;
    if (!istAdmin) return;
    void upload(event.dataTransfer?.files ?? null);
  }

  onMount(() => {
    void load();
  });
</script>

<section class="flaeche overflow-hidden">
  <header class="flex items-center justify-between gap-2 p-5 pb-3">
    <div class="min-w-0">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <FileText class="h-5 w-5 shrink-0" /> Dateien
      </h2>
      <p class="truncate text-sm text-muted-foreground">
        {#if loading}
          Lade…
        {:else if dateien.length === 0}
          Noch nichts abgelegt
        {:else}
          {dateien.length}
          {dateien.length === 1 ? 'Datei' : 'Dateien'} · {groesse(listing?.used_bytes ?? 0)}
        {/if}
      </p>
    </div>

    {#if istAdmin}
      <button
        class="btn-primary shrink-0 px-3"
        onclick={() => fileInput?.click()}
        disabled={uploading}
        aria-label="Dateien hochladen"
      >
        <Upload class="h-5 w-5" />
      </button>
    {/if}
  </header>

  <input
    bind:this={fileInput}
    type="file"
    multiple
    class="hidden"
    onchange={(e) => upload((e.currentTarget as HTMLInputElement).files)}
  />

  {#if message || error}
    <div class="px-5 pb-2">
      {#if message}
        <p class="rounded-lg bg-success/10 px-3 py-2 text-sm text-success">{message}</p>
      {/if}
      {#if error}
        <p
          class="mt-1 flex items-start justify-between gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive"
        >
          <span class="min-w-0 flex-1">{error}</span>
          <button onclick={() => (error = '')} aria-label="Schließen">
            <X class="h-4 w-4 shrink-0" />
          </button>
        </p>
      {/if}
    </div>
  {/if}

  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="relative px-5 pb-5 transition-colors {dragging ? 'ring-2 ring-inset ring-primary' : ''}"
    ondragover={(e) => {
      if (!istAdmin) return;
      e.preventDefault();
      dragging = true;
    }}
    ondragleave={() => (dragging = false)}
    ondrop={onDrop}
  >
    {#if uploading}
      <div class="mb-3 flex items-center gap-3 rounded-lg bg-muted/40 px-3 py-2.5">
        <Upload class="h-5 w-5 shrink-0 text-primary" />
        <div class="h-2 flex-1 overflow-hidden rounded-full bg-muted">
          <div class="h-full bg-primary transition-all" style="width: {progress}%"></div>
        </div>
        <span class="shrink-0 text-sm tabular-nums text-muted-foreground">{progress}%</span>
      </div>
    {/if}

    {#if dragging}
      <div
        class="mb-3 flex items-center justify-center gap-2 rounded-lg bg-primary/10 px-3 py-4 text-sm font-medium text-primary"
      >
        <Upload class="h-5 w-5" /> Dateien hier ablegen
      </div>
    {/if}

    {#if loading}
      <div class="space-y-2">
        {#each Array(3) as _, i (i)}
          <div class="h-12 animate-pulse rounded-lg bg-muted/30"></div>
        {/each}
      </div>
    {:else if dateien.length === 0}
      {#if istAdmin}
        <button
          class="flex w-full flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border p-8 text-center text-muted-foreground transition-colors hover:bg-accent/40"
          onclick={() => fileInput?.click()}
        >
          <Upload class="h-9 w-9 opacity-40" />
          <span class="text-sm font-medium">Dateien hinzufügen</span>
          <span class="text-xs">Antippen, oder Dateien einfach hierher ziehen</span>
          <span class="text-xs">
            Anleitungen, Formulare, Elternbriefe · bis
            {groesse(listing?.max_file_bytes ?? 0)} je Datei
          </span>
        </button>
      {:else}
        <p class="py-8 text-center text-sm text-muted-foreground">
          Hier legt ein Elternteil Anleitungen und Formulare ab.
        </p>
      {/if}
    {:else}
      <ul class="scrollbar-thin max-h-80 space-y-1.5 overflow-y-auto pr-1">
        {#each dateien as datei (datei.name)}
          <li class="flex items-center gap-2 rounded-lg bg-muted/30 p-2.5">
            <!--
              Ein echter Link statt eines Knopfes: Der Browser lädt selbst
              herunter, zeigt seinen eigenen Fortschritt und kommt auch mit
              einer 100-MB-Datei zurecht.
            -->
            <a
              href={filesApi.url(datei.name)}
              download={datei.name}
              class="flex min-w-0 flex-1 items-center gap-2.5 transition-colors hover:text-primary"
            >
              <Download class="h-4 w-4 shrink-0 text-muted-foreground" />
              <span class="min-w-0 flex-1">
                <span class="block truncate text-sm font-medium">{datei.name}</span>
                <span class="block text-[11px] text-muted-foreground">
                  {groesse(datei.size)} · {datum(datei.modified)}
                </span>
              </span>
            </a>

            {#if istAdmin}
              <button
                class="touch-target shrink-0 text-muted-foreground hover:text-destructive"
                onclick={() => entfernen(datei)}
                aria-label="{datei.name} löschen"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}

    <!--
      Auf einem Pi ist der Platz endlich. Der Hinweis kommt erst, wenn es
      wirklich eng wird — eine dauerhaft sichtbare Speicheranzeige wäre nur
      Lärm.
    -->
    {#if listing?.tight}
      <p
        class="mt-3 flex items-start gap-2 rounded-lg bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400"
      >
        <TriangleAlert class="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span>
          Auf dem Datenträger sind nur noch {groesse(listing.free_bytes)} frei. Zeit,
          etwas aufzuräumen.
        </span>
      </p>
    {:else if istAdmin && (listing?.free_bytes ?? 0) > 0 && dateien.length > 0}
      <p class="mt-3 flex items-center gap-1.5 text-[11px] text-muted-foreground">
        <HardDrive class="h-3 w-3 shrink-0" />
        {groesse(listing?.free_bytes ?? 0)} frei
      </p>
    {/if}
  </div>
</section>
