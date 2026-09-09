<script lang="ts">
  import {
    CalendarClock, Check, CircleAlert, ListChecks, Pencil, Plus, Trash2,
  } from 'lucide-svelte';
  import { format, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ApiError, choresApi } from '$lib/api';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import type { Chore, User } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';

  let {
    chores = $bindable([]),
    users = [],
    onRefresh,
  }: {
    chores: Chore[];
    users?: User[];
    onRefresh: () => Promise<void>;
  } = $props();

  let showForm = $state(false);
  let editingId = $state<number | null>(null);
  let error = $state('');
  let completing = $state<number | null>(null);

  function emptyDraft() {
    // 'rotate' | 'everyone' | 'nobody' | die ID einer Person als Text
    return { title: '', description: '', interval_days: 7, points: 10, zustaendig: 'rotate' };
  }

  let draft = $state(emptyDraft());

  const isAdmin = $derived($session.user?.role === 'admin');

  const overdue = $derived(chores.filter((c) => c.is_overdue));
  const dueToday = $derived(chores.filter((c) => c.is_due && !c.is_overdue));
  // Alles, was gerade nicht ansteht — meist frisch erledigt.
  const later = $derived(chores.filter((c) => !c.is_due));
  const mine = $derived(chores.filter((c) => c.assignee_id === $session.user?.id));

  async function complete(chore: Chore) {
    if (completing !== null) return;
    // Erledigtes bleibt erledigt: sonst holt sich der Nächste dieselben
    // Punkte für denselben Müllsack. Der Server weist es ohnehin ab, hier
    // sparen wir die Fehlermeldung.
    if (!chore.is_due) return;
    completing = chore.id;
    error = '';
    try {
      const result = await choresApi.complete(chore.id);
      await onRefresh();
      // The shared board handles the confetti and the level-up check.
      if ($session.user) {
        await board.award($session.user.id, result.points_awarded, result.title || chore.title);
      }
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht abhaken';
      // Hat jemand anderes am anderen Gerät zuerst abgehakt, ist unsere
      // Liste veraltet — neu laden, damit die Zeile stimmt.
      if (e instanceof ApiError && e.status === 409) await onRefresh();
    } finally {
      completing = null;
    }
  }

  function startNew() {
    editingId = null;
    draft = emptyDraft();
    error = '';
    showForm = true;
  }

  function startEdit(chore: Chore) {
    editingId = chore.id;
    draft = {
      title: chore.title,
      description: chore.description,
      interval_days: chore.interval_days,
      points: chore.points,
      zustaendig:
        chore.assignment === 'person' && chore.assignee_id
          ? String(chore.assignee_id)
          : chore.assignment || 'rotate',
    };
    error = '';
    showForm = true;
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    if (!draft.title.trim()) return;
    error = '';
    // Eine Zahl im Auswahlfeld ist eine Person, alles andere eine der
    // festen Zuständigkeiten.
    const person = Number(draft.zustaendig);
    const payload = {
      title: draft.title.trim(),
      description: draft.description,
      interval_days: draft.interval_days,
      points: draft.points,
      assignment: Number.isFinite(person) && person > 0 ? 'person' : draft.zustaendig,
      assignee_id: Number.isFinite(person) && person > 0 ? person : 0,
    };
    try {
      if (editingId !== null) await choresApi.update(editingId, payload);
      else await choresApi.create(payload);
      draft = emptyDraft();
      showForm = false;
      editingId = null;
      await onRefresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht speichern';
    }
  }

  async function remove(chore: Chore) {
    const ok = await confirmAction({
      title: `„${chore.title}“ löschen?`,
      message: 'Die Aufgabe verschwindet für alle. Bereits vergebene Punkte bleiben.',
    });
    if (!ok) return;
    try {
      await choresApi.remove(chore.id);
      await onRefresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht löschen';
    }
  }

  // "Nicht fällig" hat zwei ganz verschiedene Gründe: erledigt, oder schlicht
  // noch nicht an der Reihe. Eine nie erledigte Aufgabe als "erledigt" zu
  // zeigen wäre schlicht falsch.
  function isDone(chore: Chore): boolean {
    return !chore.is_due && !!chore.last_done_at;
  }

  function dueLabel(chore: Chore): string {
    if (!chore.next_due_at) return 'Kein Termin';
    if (chore.is_overdue) {
      const days = Math.abs(chore.days_until_due);
      return days >= 1 ? `${days} Tage überfällig` : 'Überfällig';
    }
    if (chore.is_due) return 'Heute fällig';

    const wieder = isDone(chore) ? 'Wieder ' : '';
    if (chore.days_until_due === 1) return `${wieder}morgen`;
    const tag = format(parseISO(chore.next_due_at), 'EEEE, d. MMM', { locale: de });
    return wieder ? `${wieder}${tag}` : tag;
  }

  // Wer ist zuständig? "Alle" und "Wer mag" sind bewusste Antworten und
  // nicht dasselbe wie eine leere Zeile.
  function whoLabel(chore: Chore): string {
    if (chore.assignment === 'everyone') return '👪 Alle';
    if (chore.assignment === 'nobody') return '🙋 Wer mag';
    if (chore.assignee_name) return `${chore.assignee_emoji} ${chore.assignee_name}`;
    return '🔄 Reihum';
  }

  // Wer zuletzt dran war. Steht nur an erledigten Zeilen, dort ist es die
  // Antwort auf die naheliegende Frage "warum kann ich das nicht abhaken?".
  function doneLabel(chore: Chore): string {
    return chore.last_done_by ? `Erledigt von ${chore.last_done_by}` : 'Erledigt';
  }
</script>

<section class="flaeche">
  <header class="mb-4 flex items-start justify-between gap-2">
    <div class="min-w-0">
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <ListChecks class="h-5 w-5 shrink-0" /> Aufgaben
      </h2>
      <p class="text-sm text-muted-foreground">
        {#if overdue.length > 0}
          <span class="text-destructive">{overdue.length} überfällig</span>
        {:else if dueToday.length > 0}
          {dueToday.length} heute fällig
        {:else}
          Alles im Plan
        {/if}
        {#if mine.length > 0}· {mine.length} für dich{/if}
      </p>
    </div>
    {#if isAdmin}
      <button
        class="btn-primary shrink-0 px-3"
        onclick={startNew}
        aria-label="Aufgabe hinzufügen"
      >
        <Plus class="h-5 w-5" />
      </button>
    {/if}
  </header>


  {#if error}
    <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
  {/if}

  <!-- Das Formular liegt als Fenster über der Seite. Inline würde es die
       Kachel bei jedem Anlegen um die halbe Höhe aufblähen. -->
  <Modal bind:open={showForm} title={editingId !== null ? 'Aufgabe bearbeiten' : 'Neue Aufgabe'}>
    <form class="space-y-2" onsubmit={save}>
      <input class="input" placeholder="Was ist zu tun?" bind:value={draft.title} maxlength="80" />
      <input class="input" placeholder="Beschreibung (optional)" bind:value={draft.description} />
      <!-- Zwei Zahlen nebeneinander, die Zuständigkeit auf voller Breite:
           in drei Spalten war das Auswahlfeld so schmal, dass "Reihum"
           abgeschnitten wurde. -->
      <div class="grid grid-cols-2 gap-2">
        <label class="text-xs text-muted-foreground">
          Alle X Tage
          <input class="input mt-1" type="number" min="1" max="365" bind:value={draft.interval_days} />
        </label>
        <label class="text-xs text-muted-foreground">
          Punkte
          <input class="input mt-1" type="number" min="1" max="500" bind:value={draft.points} />
        </label>
      </div>
      <label class="block text-xs text-muted-foreground">
          Zuständig
          <select class="input mt-1" bind:value={draft.zustaendig}>
            <option value="rotate">🔄 Reihum</option>
            <option value="everyone">👪 Alle</option>
            <option value="nobody">🙋 Wer mag</option>
            {#each users as u (u.id)}
              <option value={String(u.id)}>{u.avatar_emoji} {u.name}</option>
            {/each}
          </select>
      </label>
      <div class="flex gap-2 pt-1">
        <button type="button" class="btn-outline flex-1" onclick={() => (showForm = false)}>
          Abbrechen
        </button>
        <button class="btn-primary flex-1" disabled={!draft.title.trim()}>
          {editingId !== null ? 'Änderungen speichern' : 'Anlegen'}
        </button>
      </div>
    </form>
  </Modal>

  {#if chores.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <ListChecks class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Noch keine Aufgaben angelegt</p>
      {#if isAdmin}
        <p class="mt-1 text-xs">Mit <strong>+</strong> die erste anlegen</p>
      {/if}
    </div>
  {:else}
    <ul class="scrollbar-thin max-h-[320px] overflow-y-auto pr-1">
      {#each [...overdue, ...dueToday, ...later] as chore (chore.id)}
        <li
          class="group flex items-center gap-3 rounded-lg px-1 py-2.5 transition-colors
                 [&+li]:border-t [&+li]:border-[color:var(--haarlinie)] hover:bg-muted/25"
        >
          {#if chore.is_due}
            <button
              class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border-2 border-primary/30 text-primary transition-all hover:border-primary hover:bg-primary hover:text-primary-foreground active:scale-90 disabled:opacity-50"
              onclick={() => complete(chore)}
              disabled={completing !== null}
              aria-label="{chore.title} erledigt"
              title="Erledigt – gibt {chore.points} Punkte"
            >
              {#if completing === chore.id}
                <span class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
              {:else}
                <span class="text-lg leading-none">✓</span>
              {/if}
            </button>
          {:else if isDone(chore)}
            <!-- Erledigt: kein Knopf, damit niemand dieselbe Aufgabe ein
                 zweites Mal abhakt und dafür noch einmal Punkte bekommt. -->
            <span
              class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-primary/15 text-primary"
              title={doneLabel(chore)}
            >
              <Check class="h-5 w-5" />
            </span>
          {:else}
            <!-- Steht erst später an, gemacht hat sie noch niemand. -->
            <span
              class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border-2 border-dashed border-muted-foreground/25 text-muted-foreground"
              title="Noch nicht an der Reihe"
            >
              <CalendarClock class="h-4 w-4" />
            </span>
          {/if}

          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium {chore.is_due ? '' : 'text-muted-foreground'}">
              {chore.title}
            </p>
            <p class="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
              {#if chore.is_overdue}<CircleAlert class="h-3 w-3 shrink-0 text-destructive" />{/if}
              {#if chore.is_due}
                <span class={chore.is_overdue ? 'text-destructive' : ''}>{dueLabel(chore)}</span>
                <span>· {whoLabel(chore)}</span>
              {:else if isDone(chore)}
                <span class="text-primary">{doneLabel(chore)}</span>
                <span>· {dueLabel(chore)}</span>
              {:else}
                <span>{dueLabel(chore)}</span>
                <span>· {whoLabel(chore)}</span>
              {/if}
            </p>
          </div>

          <span
            class="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold tabular-nums {chore.is_due
              ? 'bg-amber-500/15 text-amber-600 dark:text-amber-400'
              : 'bg-muted text-muted-foreground'}"
          >
            +{chore.points}
          </span>

          {#if isAdmin}
            <span class="row-actions">
              <button
                class="touch-target text-muted-foreground"
                onclick={() => startEdit(chore)}
                aria-label="{chore.title} bearbeiten"
              >
                <Pencil class="h-4 w-4" />
              </button>
              <button
                class="touch-target text-muted-foreground hover:text-destructive"
                onclick={() => remove(chore)}
                aria-label="{chore.title} löschen"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>
