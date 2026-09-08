<script lang="ts">
  import { onMount } from 'svelte';
  import {
    ExternalLink, Link as LinkIcon, Pencil, Pin, PinOff, Plus, Search, Trash2, Users, X,
  } from 'lucide-svelte';
  import { ApiError, linksApi } from '$lib/api';
  import { session } from '$lib/stores';
  import type { Link, LinkDraft } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  let links = $state<Link[]>([]);
  let suggested = $state<string[]>([]);
  let loading = $state(true);
  let error = $state('');
  let search = $state('');
  let showForm = $state(false);
  let editingId = $state<number | null>(null);

  const emojis = ['🔗', '📚', '🎬', '🎵', '🛒', '🏫', '🏥', '🏦', '🍕', '⚽', '🎮', '📰', '☁️', '🧾'];

  function emptyDraft(): LinkDraft {
    return {
      title: '',
      url: '',
      description: '',
      category: 'Sonstiges',
      emoji: '🔗',
      pinned: false,
      shared: false,
    };
  }

  let draft = $state<LinkDraft>(emptyDraft());

  const filtered = $derived.by(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) return links;
    return links.filter((l) =>
      [l.title, l.url, l.description, l.category].some((field) =>
        field.toLowerCase().includes(needle),
      ),
    );
  });

  /** Grouped by category, categories sorted alphabetically but "Sonstiges" last. */
  const grouped = $derived.by(() => {
    const byCategory = new Map<string, Link[]>();
    for (const link of filtered) {
      const key = link.category || 'Sonstiges';
      const bucket = byCategory.get(key);
      if (bucket) bucket.push(link);
      else byCategory.set(key, [link]);
    }
    return [...byCategory.entries()].sort(([a], [b]) => {
      if (a === 'Sonstiges') return 1;
      if (b === 'Sonstiges') return -1;
      return a.localeCompare(b, 'de');
    });
  });

  const categoryOptions = $derived([
    ...new Set([...suggested, ...links.map((l) => l.category).filter(Boolean)]),
  ]);

  async function load() {
    try {
      const data = await linksApi.list();
      links = data.links;
      suggested = data.suggested;
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Links konnten nicht geladen werden';
    } finally {
      loading = false;
    }
  }

  function startNew() {
    editingId = null;
    draft = emptyDraft();
    error = '';
    showForm = true;
  }

  function startEdit(link: Link) {
    editingId = link.id;
    draft = {
      title: link.title,
      url: link.url,
      description: link.description,
      category: link.category,
      emoji: link.emoji,
      pinned: link.pinned,
      shared: link.shared,
    };
    error = '';
    showForm = true;
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!draft.title.trim() || !draft.url.trim()) return;
    error = '';
    try {
      if (editingId !== null) {
        const updated = await linksApi.update(editingId, draft);
        links = links.map((l) => (l.id === updated.id ? updated : l));
      } else {
        links = [...links, await linksApi.create(draft)];
      }
      showForm = false;
      editingId = null;
      draft = emptyDraft();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Link konnte nicht gespeichert werden';
    }
  }

  async function togglePin(link: Link) {
    try {
      const updated = await linksApi.togglePin(link.id);
      links = links.map((l) => (l.id === updated.id ? updated : l));
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht anpinnen';
    }
  }

  async function remove(link: Link) {
    const ok = await confirmAction({ title: `„${link.title}“ löschen?` });
    if (!ok) return;
    try {
      await linksApi.remove(link.id);
      links = links.filter((l) => l.id !== link.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht löschen';
    }
  }

  /** Strip the scheme so the list stays readable. */
  const host = (url: string) => {
    try {
      return new URL(url).host.replace(/^www\./, '');
    } catch {
      return url;
    }
  };

  onMount(load);
</script>

<svelte:head><title>Links · Familien Dashboard</title></svelte:head>

<div class="mx-auto max-w-3xl px-4 py-5">
  <header class="mb-5 flex items-start justify-between gap-3">
    <div>
      <h1 class="flex items-center gap-2 text-2xl font-semibold">
        <LinkIcon class="h-6 w-6" /> Links
      </h1>
      <p class="text-sm text-muted-foreground">
        Deine Lesezeichen. Angepinnte erscheinen auf der Übersicht.
      </p>
    </div>
    <button class="btn-primary shrink-0 px-3" onclick={() => (showForm ? (showForm = false) : startNew())}>
      {#if showForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
    </button>
  </header>

  {#if error}
    <p class="mb-4 rounded-xl bg-destructive/10 px-4 py-2.5 text-sm text-destructive">{error}</p>
  {/if}

  {#if showForm}
    <form class="card mb-4 space-y-3 p-4" onsubmit={save}>
      <div class="flex gap-2">
        <select class="input w-20 shrink-0 text-center text-xl" bind:value={draft.emoji} aria-label="Symbol">
          {#each emojis as emoji}<option value={emoji}>{emoji}</option>{/each}
        </select>
        <input class="input flex-1" placeholder="Name des Links" bind:value={draft.title} maxlength="80" />
      </div>

      <input
        class="input"
        placeholder="Adresse, z. B. wetter.orf.at"
        bind:value={draft.url}
        inputmode="url"
        autocapitalize="off"
        spellcheck="false"
      />
      <p class="text-xs text-muted-foreground">
        „https://" darf fehlen – das ergänzen wir automatisch.
      </p>

      <input class="input" placeholder="Notiz (optional)" bind:value={draft.description} maxlength="200" />

      <label class="block text-sm">
        Kategorie
        <input
          class="input mt-1"
          list="link-categories"
          bind:value={draft.category}
          placeholder="Sonstiges"
          maxlength="40"
        />
        <datalist id="link-categories">
          {#each categoryOptions as category}<option value={category}></option>{/each}
        </datalist>
      </label>

      <div class="flex flex-wrap gap-4 text-sm">
        <label class="flex items-center gap-2">
          <input type="checkbox" class="h-4 w-4 rounded" bind:checked={draft.pinned} />
          Auf der Übersicht anzeigen
        </label>
        <label class="flex items-center gap-2">
          <input type="checkbox" class="h-4 w-4 rounded" bind:checked={draft.shared} />
          Für die ganze Familie
        </label>
      </div>

      <button class="btn-primary w-full" disabled={!draft.title.trim() || !draft.url.trim()}>
        {editingId !== null ? 'Änderungen speichern' : 'Link speichern'}
      </button>
    </form>
  {/if}

  {#if links.length > 3}
    <div class="relative mb-4">
      <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <input class="input pl-9" placeholder="Links durchsuchen" bind:value={search} />
    </div>
  {/if}

  {#if loading}
    <div class="card h-48 animate-pulse bg-muted/40"></div>
  {:else if links.length === 0}
    <section class="card p-8 text-center">
      <LinkIcon class="mx-auto mb-3 h-10 w-10 text-muted-foreground opacity-40" />
      <p class="font-medium">Noch keine Links</p>
      <p class="mt-1 text-sm text-muted-foreground">
        Sammle hier, was du regelmäßig brauchst – Schulportal, Streaming, Bank.
      </p>
      <button class="btn-primary mt-4" onclick={startNew}>Ersten Link anlegen</button>
    </section>
  {:else if filtered.length === 0}
    <p class="py-8 text-center text-sm text-muted-foreground">
      Nichts gefunden für „{search}".
    </p>
  {:else}
    <div class="space-y-5">
      {#each grouped as [category, entries] (category)}
        <section>
          <h2 class="mb-2 px-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {category} · {entries.length}
          </h2>
          <div class="card divide-y divide-border">
            {#each entries as link (link.id)}
              <div class="group flex items-center gap-3 p-3">
                <a
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="flex min-w-0 flex-1 items-center gap-3"
                >
                  <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-muted text-xl">
                    {link.emoji}
                  </span>
                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-1.5 truncate text-sm font-medium">
                      {link.title}
                      {#if link.pinned}<Pin class="h-3 w-3 shrink-0 text-primary" />{/if}
                      {#if link.shared}
                        <Users class="h-3 w-3 shrink-0 text-muted-foreground" />
                      {/if}
                      <ExternalLink class="h-3 w-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
                    </span>
                    <span class="block truncate text-xs text-muted-foreground">
                      {link.description || host(link.url)}
                    </span>
                    {#if !link.editable && link.owner_name}
                      <span class="block text-[11px] text-muted-foreground">
                        geteilt von {link.owner_name}
                      </span>
                    {/if}
                  </span>
                </a>

                {#if link.editable}
                  <div class="flex shrink-0 gap-0.5">
                    <button
                      class="touch-target {link.pinned ? 'text-primary' : 'text-muted-foreground'}"
                      onclick={() => togglePin(link)}
                      aria-label={link.pinned ? 'Von der Übersicht nehmen' : 'Auf der Übersicht anzeigen'}
                      title={link.pinned ? 'Von der Übersicht nehmen' : 'Auf der Übersicht anzeigen'}
                    >
                      {#if link.pinned}<PinOff class="h-4 w-4" />{:else}<Pin class="h-4 w-4" />{/if}
                    </button>
                    <button
                      class="touch-target text-muted-foreground"
                      onclick={() => startEdit(link)}
                      aria-label="{link.title} bearbeiten"
                    >
                      <Pencil class="h-4 w-4" />
                    </button>
                    <button
                      class="touch-target text-muted-foreground hover:text-destructive"
                      onclick={() => remove(link)}
                      aria-label="{link.title} löschen"
                    >
                      <Trash2 class="h-4 w-4" />
                    </button>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {/if}
</div>
