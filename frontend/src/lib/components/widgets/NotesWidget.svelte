<script lang="ts">
  import { FileText, Pin, PinOff, Plus, Trash2, X } from 'lucide-svelte';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { ApiError, notesApi } from '$lib/api';
  import type { Note } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  let { notes = $bindable([]) }: { notes: Note[] } = $props();

  let showForm = $state(false);
  let editing = $state<Note | null>(null);
  let expanded = $state<number | null>(null);
  let error = $state('');
  let draft = $state({ title: '', content: '', tags: '', pinned: false, shared: true });

  const sorted = $derived(
    [...notes].sort(
      (a, b) =>
        Number(b.pinned) - Number(a.pinned) ||
        b.updated_at.localeCompare(a.updated_at),
    ),
  );

  // Notes are Markdown written by the family; sanitise before rendering so a
  // pasted snippet can never inject script into the dashboard.
  function render(markdown: string): string {
    const html = marked.parse(markdown, { async: false, breaks: true }) as string;
    return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
  }

  function startNew() {
    editing = null;
    draft = { title: '', content: '', tags: '', pinned: false, shared: true };
    showForm = true;
  }

  function startEdit(note: Note) {
    editing = note;
    draft = {
      title: note.title,
      content: note.content,
      tags: note.tags.join(', '),
      pinned: note.pinned,
      shared: note.owner_id === null,
    };
    showForm = true;
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!draft.title.trim()) return;
    error = '';
    const tags = draft.tags.split(',').map((t) => t.trim()).filter(Boolean);

    try {
      if (editing) {
        const updated = await notesApi.update(editing.id, {
          title: draft.title.trim(),
          content: draft.content,
          tags,
          pinned: draft.pinned,
        });
        notes = notes.map((n) => (n.id === updated.id ? updated : n));
      } else {
        const created = await notesApi.create({
          title: draft.title.trim(),
          content: draft.content,
          tags,
          pinned: draft.pinned,
          shared: draft.shared,
        });
        notes = [created, ...notes];
      }
      showForm = false;
      editing = null;
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht speichern';
    }
  }

  async function togglePin(note: Note) {
    try {
      const updated = await notesApi.update(note.id, { pinned: !note.pinned });
      notes = notes.map((n) => (n.id === updated.id ? updated : n));
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht anpinnen';
    }
  }

  async function remove(note: Note) {
    const ok = await confirmAction({
      title: `„${note.title}“ löschen?`,
      message: 'Auch die Markdown-Datei wird entfernt.',
    });
    if (!ok) return;
    try {
      await notesApi.remove(note.id);
      notes = notes.filter((n) => n.id !== note.id);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht löschen';
    }
  }
</script>

<section class="card p-5">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <FileText class="h-5 w-5" /> Notizen
      </h2>
      <p class="text-sm text-muted-foreground">{notes.length} Einträge</p>
    </div>
    <button
      class="btn-primary px-3"
      onclick={() => (showForm ? (showForm = false) : startNew())}
      aria-label="Notiz hinzufügen"
    >
      {#if showForm}<X class="h-5 w-5" />{:else}<Plus class="h-5 w-5" />{/if}
    </button>
  </header>

  {#if error}
    <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
  {/if}

  {#if showForm}
    <form class="mb-4 space-y-2 rounded-lg border border-border p-3" onsubmit={save}>
      <input class="input" placeholder="Titel" bind:value={draft.title} maxlength="120" />
      <textarea
        class="input min-h-[120px] font-mono text-sm"
        placeholder="Inhalt (Markdown)"
        bind:value={draft.content}
      ></textarea>
      <input class="input" placeholder="Tags, kommagetrennt" bind:value={draft.tags} />
      <div class="flex flex-wrap gap-4 text-sm">
        <label class="flex items-center gap-2">
          <input type="checkbox" class="h-4 w-4 rounded" bind:checked={draft.pinned} />
          Anpinnen
        </label>
        {#if !editing}
          <label class="flex items-center gap-2">
            <input type="checkbox" class="h-4 w-4 rounded" bind:checked={draft.shared} />
            Für alle sichtbar
          </label>
        {/if}
      </div>
      <button class="btn-primary w-full" disabled={!draft.title.trim()}>
        {editing ? 'Speichern' : 'Anlegen'}
      </button>
    </form>
  {/if}

  {#if notes.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <FileText class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Noch keine Notizen</p>
    </div>
  {:else}
    <ul class="scrollbar-thin max-h-[340px] space-y-2 overflow-y-auto pr-1">
      {#each sorted as note (note.id)}
        <li class="group rounded-lg bg-muted/30 p-3">
          <div class="flex items-start gap-2">
            <button
              class="min-w-0 flex-1 text-left"
              onclick={() => (expanded = expanded === note.id ? null : note.id)}
              aria-expanded={expanded === note.id}
            >
              <p class="flex items-center gap-1.5 truncate text-sm font-medium">
                {#if note.pinned}<Pin class="h-3 w-3 shrink-0 text-primary" />{/if}
                {note.title}
              </p>
              {#if note.tags.length > 0}
                <div class="mt-1 flex flex-wrap gap-1">
                  {#each note.tags as tag}
                    <span class="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] text-primary">
                      {tag}
                    </span>
                  {/each}
                </div>
              {/if}
            </button>

            <div class="row-actions">
              <button
                class="touch-target text-muted-foreground"
                onclick={() => togglePin(note)}
                aria-label={note.pinned ? 'Loslösen' : 'Anpinnen'}
              >
                {#if note.pinned}<PinOff class="h-4 w-4" />{:else}<Pin class="h-4 w-4" />{/if}
              </button>
              <button
                class="touch-target text-muted-foreground"
                onclick={() => startEdit(note)}
                aria-label="Bearbeiten"
              >
                <FileText class="h-4 w-4" />
              </button>
              <button
                class="touch-target text-muted-foreground"
                onclick={() => remove(note)}
                aria-label="Löschen"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </div>

          {#if expanded === note.id}
            <div class="prose-sm mt-3 border-t border-border pt-3 text-sm leading-relaxed">
              {@html render(note.content)}
            </div>
          {:else if note.content}
            <p class="mt-1 line-clamp-2 text-xs text-muted-foreground">
              {note.content.replace(/[#*_`>-]/g, '').slice(0, 160)}
            </p>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  /* marked output needs a little structure back — Tailwind's preflight strips it. */
  .prose-sm :global(h1),
  .prose-sm :global(h2),
  .prose-sm :global(h3) {
    font-weight: 600;
    margin: 0.75em 0 0.35em;
  }
  .prose-sm :global(h1) { font-size: 1.15rem; }
  .prose-sm :global(h2) { font-size: 1.05rem; }
  .prose-sm :global(p) { margin: 0.5em 0; }
  .prose-sm :global(ul) { list-style: disc; padding-left: 1.25rem; margin: 0.5em 0; }
  .prose-sm :global(ol) { list-style: decimal; padding-left: 1.25rem; margin: 0.5em 0; }
  .prose-sm :global(code) {
    background: hsl(var(--muted));
    padding: 0.1em 0.35em;
    border-radius: 4px;
    font-size: 0.9em;
  }
  .prose-sm :global(blockquote) {
    border-left: 3px solid hsl(var(--border));
    padding-left: 0.75rem;
    color: hsl(var(--muted-foreground));
    margin: 0.5em 0;
  }
  .prose-sm :global(a) { color: hsl(var(--primary)); text-decoration: underline; }
  .prose-sm :global(hr) { border-color: hsl(var(--border)); margin: 0.75em 0; }
</style>
