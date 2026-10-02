<script lang="ts">
  import { untrack } from 'svelte';
  import { schnell } from '$lib/stores/schnell.svelte';
  import {
    CalendarClock, Check, CircleAlert, Eraser, Eye, Hourglass, ListChecks, Pencil, Plus, Trash2, X,
  } from 'lucide-svelte';
  import { format, parseISO } from 'date-fns';
  import { de } from 'date-fns/locale';
  import { ApiError, choresApi, pendingApi } from '$lib/api';
  import { eltern } from '$lib/stores/eltern.svelte';
  import { toast } from '$lib/stores/toast.svelte';
  import { session } from '$lib/stores';
  import { board } from '$lib/stores/scores.svelte';
  import Modal from '$lib/components/Modal.svelte';
  import type { Chore, PendingCompletion, User } from '$lib/types';
  import { confirmAction } from '$lib/stores/confirm.svelte';
  import Kachel from './Kachel.svelte';
  import KachelLeer from './KachelLeer.svelte';
  import { werWarDas } from '$lib/stores/werwardas.svelte';

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
    return {
      title: '',
      description: '',
      interval_days: 7,
      points: 10,
      zustaendig: 'rotate',
      einmalig: false,
      pruefen: false,
    };
  }

  let draft = $state(emptyDraft());

  const isAdmin = $derived($session.user?.role === 'admin');

  const overdue = $derived(chores.filter((c) => c.is_overdue));
  const dueToday = $derived(chores.filter((c) => c.is_due && !c.is_overdue));
  // Alles, was gerade nicht ansteht — meist frisch erledigt.
  const later = $derived(chores.filter((c) => !c.is_due && !c.done));
  // Einmalige Aufgaben, die durch sind. Sie stehen ganz unten und bleiben
  // sichtbar, bis jemand aufräumt: Am Monatsende will man sehen, was
  // tatsächlich geschafft wurde.
  const abgehakt = $derived(chores.filter((c) => c.done));
  const mine = $derived(chores.filter((c) => c.assignee_id === $session.user?.id));

  // ---- Zu bestätigen ----
  /**
   * Was Kinder als erledigt gemeldet haben und auf ein Elternteil wartet.
   * Sehen dürfen es alle — am Wandgerät ist „wartet auf Mama" genau die
   * richtige Antwort auf „wo sind meine Punkte?". Bestätigen dürfen Eltern,
   * am Wandgerät mit ihrer PIN.
   */
  let offene = $state<PendingCompletion[]>([]);
  let entscheidet = $state<number | null>(null);
  const darfBestaetigen = $derived(isAdmin || $session.device);

  async function ladeOffene() {
    try {
      offene = await pendingApi.list();
    } catch {
      offene = [];
    }
  }

  // Mit jeder neu geladenen Aufgabenliste auch die offenen Bestätigungen.
  $effect(() => {
    void chores;
    void ladeOffene();
  });

  async function bestaetigen(p: PendingCompletion) {
    if (entscheidet !== null) return;
    if (!(await eltern.brauche(`${p.user_name}: „${p.title}“ bestätigen`))) return;
    entscheidet = p.completion_id;
    try {
      const r = await pendingApi.approve(p.completion_id);
      offene = offene.filter((x) => x.completion_id !== p.completion_id);
      await onRefresh();
      await board.award(r.user_id, r.points_awarded, `${p.user_name}: ${r.title}`);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht bestätigen';
      void ladeOffene();
    } finally {
      entscheidet = null;
    }
  }

  async function ablehnen(p: PendingCompletion) {
    if (entscheidet !== null) return;
    if (!(await eltern.brauche(`${p.user_name}: „${p.title}“ ablehnen`))) return;
    entscheidet = p.completion_id;
    try {
      await pendingApi.reject(p.completion_id);
      offene = offene.filter((x) => x.completion_id !== p.completion_id);
      await onRefresh();
      toast(`„${p.title}“ ist wieder offen — noch einmal ran, ${p.user_name}.`);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht ablehnen';
      void ladeOffene();
    } finally {
      entscheidet = null;
    }
  }

  async function complete(chore: Chore) {
    if (completing !== null) return;
    // Erledigtes bleibt erledigt: sonst holt sich der Nächste dieselben
    // Punkte für denselben Müllsack. Der Server weist es ohnehin ab, hier
    // sparen wir die Fehlermeldung.
    if (!chore.is_due) return;

    // Am Wandtablet ist niemand angemeldet: erst fragen, wer abgehakt hat,
    // sonst bekäme die Punkte, wer zuletzt am Gerät war.
    let wer: number | null = $session.user?.id ?? null;
    if ($session.device) {
      wer = await werWarDas({ was: chore.title });
      if (wer === null) return;
    }

    completing = chore.id;
    error = '';
    try {
      const result = await choresApi.complete(chore.id, $session.device ? wer! : undefined);
      await onRefresh();
      if (result.pending) {
        // Erledigt, aber die Punkte warten auf ein Elternteil. Kein Konfetti
        // für etwas, das noch nicht feststeht — aber ein klares Wort.
        toast(`Erledigt! Die ${result.points_pending ?? chore.points} Punkte gibt es, sobald Mama oder Papa nachgesehen hat.`, {
          ton: 'erfolg',
          dauer: 5000,
        });
        void ladeOffene();
      } else if (wer !== null) {
        // The shared board handles the confetti and the level-up check.
        await board.award(wer, result.points_awarded, result.title || chore.title);
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

  // Das Plus unten hat nach diesem Formular gefragt.
  $effect(() => {
    if (schnell.anfrage !== 'aufgabe') return;
    untrack(() => {
      schnell.abholen('aufgabe');
      startNew();
    });
  });

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
      einmalig: chore.one_off,
      pruefen: chore.needs_check,
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
      one_off: draft.einmalig,
      needs_check: draft.pruefen,
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

  // Erledigte einmalige Aufgaben wegräumen — alle auf einmal. Einzeln
  // wären es am Monatsende zwanzig Klicks.
  async function aufraeumen() {
    const ok = await confirmAction({
      title: `${abgehakt.length} erledigte Aufgaben löschen?`,
      message:
        'Nur einmalige Aufgaben, die abgehakt sind. Die vergebenen Punkte und die ' +
        'Monatsranglisten bleiben davon unberührt.',
    });
    if (!ok) return;
    try {
      await choresApi.clearDone();
      await onRefresh();
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Konnte nicht aufräumen';
    }
  }

  // "Nicht fällig" hat zwei ganz verschiedene Gründe: erledigt, oder schlicht
  // noch nicht an der Reihe. Eine nie erledigte Aufgabe als "erledigt" zu
  // zeigen wäre schlicht falsch.
  function isDone(chore: Chore): boolean {
    return chore.done || (!chore.is_due && !!chore.last_done_at);
  }

  function dueLabel(chore: Chore): string {
    // Eine erledigte einmalige Aufgabe kommt nicht wieder. „Wieder am
    // Donnerstag" wäre hier schlicht gelogen.
    if (chore.done) {
      return chore.last_done_at
        ? `Am ${format(parseISO(chore.last_done_at), 'd. MMM', { locale: de })}`
        : 'Erledigt';
    }
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
    if (chore.pending_check) {
      return chore.last_done_by ? `${chore.last_done_by} · wartet auf Bestätigung` : 'Wartet auf Bestätigung';
    }
    return chore.last_done_by ? `Erledigt von ${chore.last_done_by}` : 'Erledigt';
  }
</script>

{#snippet zeile()}
  {#if chores.length === 0}
    Noch nichts angelegt
  {:else}
    {#if overdue.length > 0}
      <span class="text-destructive">{overdue.length} überfällig</span>
    {:else if dueToday.length > 0}
      {dueToday.length} heute fällig
    {:else}
      Alles im Plan
    {/if}
    {#if mine.length > 0}· {mine.length} für dich{/if}
    {#if abgehakt.length > 0}· {abgehakt.length} erledigt{/if}
  {/if}
{/snippet}

{#snippet aktionen()}
  {#if isAdmin}
    {#if abgehakt.length > 0}
      <button
        class="btn-outline px-3"
        onclick={aufraeumen}
        aria-label="Erledigte einmalige Aufgaben löschen"
        title="{abgehakt.length} erledigte einmalige Aufgaben löschen"
      >
        <Eraser class="h-5 w-5" />
      </button>
    {/if}
    <button class="btn-primary px-3" onclick={startNew} aria-label="Aufgabe hinzufügen">
      <Plus class="h-5 w-5" />
    </button>
  {/if}
{/snippet}

<Kachel ton="var(--ton-aufgaben)" titel="Aufgaben" icon={ListChecks} {zeile} {aktionen}>
  {#if error}
    <p class="mb-3 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
  {/if}

  <!-- Das Formular liegt als Fenster über der Seite. Inline würde es die
       Kachel bei jedem Anlegen um die halbe Höhe aufblähen. -->
  <Modal bind:open={showForm} title={editingId !== null ? 'Aufgabe bearbeiten' : 'Neue Aufgabe'}>
    <form class="space-y-2" onsubmit={save}>
      <input class="input" placeholder="Was ist zu tun?" bind:value={draft.title} maxlength="80" />
      <input class="input" placeholder="Beschreibung (optional)" bind:value={draft.description} />
      <!--
        Einmalig oder wiederkehrend. Ein Intervall für „Keller aufräumen"
        wäre eine Erfindung: Die Aufgabe kommt nicht in sieben Tagen wieder,
        sie ist dann fertig. Also verschwindet das Feld, statt daneben zu
        stehen und nichts zu bedeuten.
      -->
      <label class="flex items-start gap-2 rounded-lg border border-border p-2.5 text-sm">
        <input type="checkbox" class="mt-0.5 h-4 w-4 rounded" bind:checked={draft.einmalig} />
        <span>
          Nur einmal
          <span class="block text-xs text-muted-foreground">
            Steht ab sofort an und ist nach dem Abhaken erledigt — zum Beispiel
            ein Zahnarzttermin oder der Keller.
          </span>
        </span>
      </label>
      <!-- Eltern bestätigen: Für „Zimmer aufräumen" lohnt sich ein Blick,
           bevor die Punkte fliessen. Eltern selbst bekommen ihre sofort. -->
      <label class="flex items-start gap-2 rounded-lg border border-border p-2.5 text-sm">
        <input type="checkbox" class="mt-0.5 h-4 w-4 rounded" bind:checked={draft.pruefen} />
        <span>
          Eltern bestätigen
          <span class="block text-xs text-muted-foreground">
            Kinder bekommen die Punkte erst, wenn Mama oder Papa nachgesehen
            hat. Bis dahin steht die Aufgabe als „wartet" da.
          </span>
        </span>
      </label>
      <!-- Zwei Zahlen nebeneinander, die Zuständigkeit auf voller Breite:
           in drei Spalten war das Auswahlfeld so schmal, dass "Reihum"
           abgeschnitten wurde. -->
      <div class="grid grid-cols-2 gap-2">
        {#if !draft.einmalig}
          <label class="text-xs text-muted-foreground">
            Alle X Tage
            <input class="input mt-1" type="number" min="1" max="365" bind:value={draft.interval_days} />
          </label>
        {/if}
        <label class="text-xs text-muted-foreground {draft.einmalig ? 'col-span-2' : ''}">
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

  {#if offene.length > 0}
    <section class="mb-3 rounded-xl border border-amber-500/30 bg-amber-500/5 p-2.5" aria-label="Zu bestätigen">
      <p class="mb-1.5 flex items-center gap-1.5 px-1 text-xs font-semibold uppercase tracking-wider text-amber-700 dark:text-amber-400">
        <Eye class="h-3.5 w-3.5" /> Zu bestätigen · {offene.length}
      </p>
      <ul class="space-y-1">
        {#each offene as p (p.completion_id)}
          <li class="flex items-center gap-2 rounded-lg px-1 py-1.5">
            <span class="text-xl">{p.user_emoji}</span>
            <div class="min-w-0 flex-1">
              <p class="truncate text-sm font-medium">{p.title}</p>
              <p class="truncate text-xs text-muted-foreground">{p.user_name} · +{p.points}</p>
            </div>
            {#if darfBestaetigen}
              <button
                class="touch-target shrink-0 rounded-full text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                onclick={() => ablehnen(p)}
                disabled={entscheidet !== null}
                aria-label="{p.title} von {p.user_name} ablehnen"
                title="Noch nicht — wieder offen"
              >
                <X class="h-5 w-5" />
              </button>
              <button
                class="flex h-11 shrink-0 items-center gap-1 rounded-full bg-success px-3 text-sm font-semibold text-success-foreground hover:bg-success/90 disabled:opacity-50"
                onclick={() => bestaetigen(p)}
                disabled={entscheidet !== null}
                aria-label="{p.title} von {p.user_name} bestätigen"
              >
                <Check class="h-4 w-4" /> +{p.points}
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  {#if chores.length === 0}
    <KachelLeer
      icon={ListChecks}
      titel="Noch keine Aufgaben angelegt"
      hinweis={isAdmin
        ? 'Mit + die erste anlegen. Jede Aufgabe hat ein Intervall und einen Punktwert.'
        : 'Ein Elternteil legt die Aufgaben an.'}
    />
  {:else}
    <ul class="scrollbar-thin max-h-[320px] overflow-y-auto pr-1">
      {#each [...overdue, ...dueToday, ...later, ...abgehakt] as chore (chore.id)}
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
          {:else if chore.pending_check}
            <!-- Erledigt gemeldet, wartet auf ein Elternteil. -->
            <span
              class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-amber-500/15 text-amber-600 dark:text-amber-400"
              title={doneLabel(chore)}
            >
              <Hourglass class="h-5 w-5" />
            </span>
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
            <!-- Durchgestrichen nur, was endgültig durch ist. Eine
                 wiederkehrende Aufgabe ist nach dem Abhaken nicht erledigt,
                 sondern nur heute nicht mehr dran. -->
            <p
              class="truncate text-sm font-medium {chore.is_due
                ? ''
                : 'text-muted-foreground'} {chore.done ? 'line-through' : ''}"
            >
              {chore.title}
            </p>
            <p class="flex flex-wrap items-center gap-x-1.5 text-xs text-muted-foreground">
              {#if chore.is_overdue}<CircleAlert class="h-3 w-3 shrink-0 text-destructive" />{/if}
              {#if chore.done}
                <span class="text-primary">{doneLabel(chore)}</span>
                <span>· {dueLabel(chore)}</span>
              {:else if chore.is_due}
                <span class={chore.is_overdue ? 'text-destructive' : ''}>{dueLabel(chore)}</span>
                <span>· {whoLabel(chore)}</span>
                {#if chore.one_off}<span>· einmalig</span>{/if}
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
</Kachel>
