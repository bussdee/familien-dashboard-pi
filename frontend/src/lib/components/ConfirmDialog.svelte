<script lang="ts">
  import { TriangleAlert } from 'lucide-svelte';
  import { confirmStore } from '$lib/stores/confirm.svelte';

  const request = $derived(confirmStore.request);
  let confirmButton = $state<HTMLButtonElement | null>(null);

  // Move focus into the dialog so Enter confirms and Escape cancels without
  // hunting for the buttons.
  $effect(() => {
    if (request) confirmButton?.focus();
  });
</script>

<svelte:window
  onkeydown={(e) => {
    if (!confirmStore.request) return;
    if (e.key === 'Escape') confirmStore.answer(false);
  }}
/>

{#if request}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed inset-0 z-[70] flex items-end justify-center bg-black/50 p-4 backdrop-blur-sm sm:items-center"
    onclick={(e) => e.target === e.currentTarget && confirmStore.answer(false)}
    role="presentation"
  >
    <div
      class="safe-bottom w-full max-w-sm animate-slide-up rounded-2xl border border-border bg-card p-5 shadow-xl"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
    >
      <div class="flex gap-3">
        {#if request.danger}
          <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-destructive/10">
            <TriangleAlert class="h-5 w-5 text-destructive" />
          </span>
        {/if}
        <div class="min-w-0 flex-1">
          <h2 id="confirm-title" class="font-semibold">{request.title}</h2>
          {#if request.message}
            <p class="mt-1 text-sm text-muted-foreground">{request.message}</p>
          {/if}
        </div>
      </div>

      <div class="mt-5 flex gap-2">
        <button class="btn-outline flex-1" onclick={() => confirmStore.answer(false)}>
          {request.cancelLabel}
        </button>
        <button
          bind:this={confirmButton}
          class="flex-1 {request.danger
            ? 'btn bg-destructive text-destructive-foreground hover:bg-destructive/90'
            : 'btn-primary'}"
          onclick={() => confirmStore.answer(true)}
        >
          {request.confirmLabel}
        </button>
      </div>
    </div>
  </div>
{/if}
