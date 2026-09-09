<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ChevronLeft, ChevronRight, Expand, Image, Pause, Play, Trash2, Upload, X,
  } from 'lucide-svelte';
  import { ApiError, photosApi } from '$lib/api';
  import { session } from '$lib/stores';
  import type { Photo } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  const ROTATE_MS = 20_000;
  const ACCEPT = 'image/jpeg,image/png,image/webp,image/gif,image/avif';

  let photos = $state<Photo[]>([]);
  let index = $state(0);
  let loading = $state(true);
  let playing = $state(true);

  let uploading = $state(false);
  let progress = $state(0);
  let message = $state('');
  let error = $state('');
  let dragging = $state(false);
  let fileInput: HTMLInputElement | null = $state(null);

  const current = $derived(photos[index] ?? null);
  const isAdmin = $derived($session.user?.role === 'admin');

  function step(delta: number) {
    if (photos.length === 0) return;
    index = (index + delta + photos.length) % photos.length;
  }

  async function load(keepPosition = false) {
    try {
      const data = await photosApi.list();
      photos = data.photos;
      if (photos.length === 0) index = 0;
      else if (!keepPosition) index = Math.floor(Math.random() * photos.length);
      else index = Math.min(index, photos.length - 1);
    } catch {
      photos = [];
    } finally {
      loading = false;
    }
  }

  async function upload(files: FileList | File[] | null) {
    const list = [...(files ?? [])];
    if (list.length === 0 || uploading) return;

    uploading = true;
    progress = 0;
    error = '';
    message = '';

    try {
      const result = await photosApi.upload(list, (percent) => (progress = percent));
      const skipped = Object.entries(result.skipped ?? {});
      message =
        result.count === 1
          ? '1 Foto hinzugefügt'
          : `${result.count} Fotos hinzugefügt`;
      if (skipped.length > 0) {
        error = skipped.map(([name, reason]) => `${name}: ${reason}`).join(' · ');
      }

      await load(true);
      // Show what was just added rather than leaving the frame where it was.
      if (result.uploaded.length > 0) {
        const first = photos.findIndex((p) => p.name === result.uploaded[0]);
        if (first >= 0) index = first;
      }
      setTimeout(() => (message = ''), 4000);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Upload fehlgeschlagen';
    } finally {
      uploading = false;
      progress = 0;
      if (fileInput) fileInput.value = '';
    }
  }

  async function removeCurrent() {
    if (!current) return;
    const ok = await confirmAction({
      title: 'Foto löschen?',
      message: `„${current.name}“ wird endgültig entfernt.`,
    });
    if (!ok) return;
    try {
      await photosApi.remove(current.name);
      await load(true);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Löschen fehlgeschlagen';
    }
  }

  function onDrop(event: DragEvent) {
    event.preventDefault();
    dragging = false;
    void upload(event.dataTransfer?.files ?? null);
  }

  onMount(() => {
    void load();
    const timer = setInterval(() => playing && !uploading && step(1), ROTATE_MS);
    return () => clearInterval(timer);
  });
</script>

<section class="flaeche overflow-hidden">
  <header class="flex items-center justify-between gap-2 p-5 pb-3">
    <div class="min-w-0">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <Image class="h-5 w-5 shrink-0" /> Foto-Rahmen
      </h2>
      <p class="truncate text-sm text-muted-foreground">
        {photos.length === 0 ? 'Keine Fotos' : `${index + 1} von ${photos.length}`}
      </p>
    </div>

    <div class="flex shrink-0 items-center gap-1">
      {#if photos.length > 0}
        <!-- Vollbild: dafür hängt das Tablet schließlich an der Wand. -->
        <a
          href="/diashow"
          class="btn-ghost px-2"
          aria-label="Diashow im Vollbild starten"
          title="Diashow im Vollbild"
        >
          <Expand class="h-5 w-5" />
        </a>
      {/if}
      {#if photos.length > 1}
        <button
          class="btn-ghost px-2"
          onclick={() => (playing = !playing)}
          aria-label={playing ? 'Pausieren' : 'Abspielen'}
        >
          {#if playing}<Pause class="h-5 w-5" />{:else}<Play class="h-5 w-5" />{/if}
        </button>
      {/if}
      <button
        class="btn-primary px-3"
        onclick={() => fileInput?.click()}
        disabled={uploading}
        aria-label="Fotos hochladen"
      >
        <Upload class="h-5 w-5" />
      </button>
    </div>
  </header>

  <input
    bind:this={fileInput}
    type="file"
    accept={ACCEPT}
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
    class="relative aspect-[4/3] bg-muted/40 transition-colors {dragging
      ? 'ring-2 ring-inset ring-primary'
      : ''}"
    ondragover={(e) => {
      e.preventDefault();
      dragging = true;
    }}
    ondragleave={() => (dragging = false)}
    ondrop={onDrop}
  >
    {#if uploading}
      <div class="absolute inset-0 z-20 flex flex-col items-center justify-center gap-3 bg-card/90">
        <Upload class="h-8 w-8 text-primary" />
        <div class="h-2 w-40 overflow-hidden rounded-full bg-muted">
          <div class="h-full bg-primary transition-all" style="width: {progress}%"></div>
        </div>
        <p class="text-sm text-muted-foreground">{progress}% übertragen</p>
      </div>
    {/if}

    {#if dragging}
      <div
        class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-2 bg-primary/10 text-primary"
      >
        <Upload class="h-8 w-8" />
        <p class="text-sm font-medium">Fotos hier ablegen</p>
      </div>
    {/if}

    {#if loading}
      <div class="absolute inset-0 flex items-center justify-center text-muted-foreground">
        <Image class="h-10 w-10 animate-pulse opacity-40" />
      </div>
    {:else if current}
      {#key current.name}
        <img
          src={photosApi.url(current.name)}
          alt={current.name}
          class="h-full w-full animate-fade-in object-cover"
          loading="lazy"
        />
      {/key}

      {#if photos.length > 1}
        <button
          class="absolute left-2 top-1/2 -translate-y-1/2 rounded-full bg-black/40 p-2 text-white backdrop-blur transition-colors hover:bg-black/60"
          onclick={() => step(-1)}
          aria-label="Vorheriges Foto"
        >
          <ChevronLeft class="h-5 w-5" />
        </button>
        <button
          class="absolute right-2 top-1/2 -translate-y-1/2 rounded-full bg-black/40 p-2 text-white backdrop-blur transition-colors hover:bg-black/60"
          onclick={() => step(1)}
          aria-label="Nächstes Foto"
        >
          <ChevronRight class="h-5 w-5" />
        </button>
      {/if}

      {#if isAdmin}
        <button
          class="absolute right-2 top-2 rounded-full bg-black/40 p-2 text-white backdrop-blur transition-colors hover:bg-destructive"
          onclick={removeCurrent}
          aria-label="Dieses Foto löschen"
          title="Dieses Foto löschen"
        >
          <Trash2 class="h-4 w-4" />
        </button>
      {/if}
    {:else}
      <button
        class="absolute inset-0 flex flex-col items-center justify-center gap-2 p-6 text-center text-muted-foreground transition-colors hover:bg-accent/40"
        onclick={() => fileInput?.click()}
      >
        <Upload class="h-10 w-10 opacity-40" />
        <p class="text-sm font-medium">Fotos hinzufügen</p>
        <p class="text-xs">Antippen, oder Bilder einfach hierher ziehen</p>
        <p class="text-xs">JPG · PNG · WEBP · GIF · AVIF, bis 25 MB je Bild</p>
      </button>
    {/if}
  </div>
</section>
