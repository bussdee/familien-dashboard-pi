<script lang="ts">
  import { onMount } from 'svelte';
  import { authApi } from '$lib/api';
  import { werWarDasStore } from '$lib/stores/werwardas.svelte';
  import type { User } from '$lib/types';

  const frage = $derived(werWarDasStore.frage);

  let users = $state<User[]>([]);

  onMount(async () => {
    try {
      users = await authApi.roster();
    } catch {
      users = [];
    }
  });
</script>

<svelte:window
  onkeydown={(e) => {
    if (werWarDasStore.frage && e.key === 'Escape') werWarDasStore.answer(null);
  }}
/>

{#if frage}
  <!--
    Bewusst groß und ohne PIN: Am Wandtablet soll ein Kind mit einem Tipp
    fertig sein. Wer sich Punkte erschleichen will, kann das — dafür ist ein
    Flurtablet der falsche Ort für strenge Kontrolle.
  -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-[75] flex items-end justify-center bg-black/55 p-0 backdrop-blur-sm sm:items-center sm:p-4"
    onclick={(e) => e.target === e.currentTarget && werWarDasStore.answer(null)}
    role="presentation"
  >
    <div
      class="safe-bottom w-full max-w-lg animate-slide-up rounded-t-3xl border border-border bg-card p-6 shadow-xl sm:rounded-3xl"
      role="dialog"
      aria-modal="true"
      aria-labelledby="wer-titel"
    >
      <h2 id="wer-titel" class="font-display text-2xl font-medium">Wer war das?</h2>
      {#if frage.was}
        <p class="mt-1 text-sm text-muted-foreground">{frage.was}</p>
      {/if}

      <div class="mt-6 grid grid-cols-3 gap-3 sm:grid-cols-4">
        {#each users as u (u.id)}
          <button
            class="flex flex-col items-center gap-2 rounded-2xl border border-[color:var(--haarlinie)] px-2 py-4 transition-colors hover:border-primary hover:bg-primary/5 active:scale-95"
            onclick={() => werWarDasStore.answer(u.id)}
          >
            <span
              class="flex h-14 w-14 items-center justify-center rounded-full text-3xl"
              style="background: {u.color}22"
            >
              {u.avatar_emoji}
            </span>
            <span class="text-sm font-medium">{u.name}</span>
          </button>
        {/each}
      </div>

      <button
        class="btn-outline mt-5 w-full"
        onclick={() => werWarDasStore.answer(null)}
      >
        Abbrechen
      </button>
    </div>
  </div>
{/if}
