<script lang="ts">
  import { board } from '$lib/stores/scores.svelte';
  import { Sparkles, Trophy } from 'lucide-svelte';

  const celebration = $derived(board.celebration);

  // Cheap confetti: a handful of coloured squares, no library, no canvas.
  const confetti = Array.from({ length: 28 }, (_, i) => ({
    left: (i * 37) % 100,
    delay: (i % 9) * 0.08,
    duration: 1.5 + ((i * 13) % 9) / 10,
    color: ['#0d9488', '#f59e0b', '#ec4899', '#3b82f6', '#22c55e'][i % 5],
    size: 6 + (i % 4) * 2,
    rotate: (i * 47) % 360,
  }));
</script>

{#if celebration}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="pointer-events-auto fixed inset-0 z-[60] flex items-center justify-center bg-background/70 backdrop-blur-sm"
    onclick={() => board.dismiss()}
    onkeydown={(e) => e.key === 'Escape' && board.dismiss()}
    role="presentation"
  >
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      {#each confetti as piece, i (i)}
        <span
          class="confetti absolute top-0 block"
          style="
            left: {piece.left}%;
            width: {piece.size}px;
            height: {piece.size * 1.6}px;
            background: {piece.color};
            animation-delay: {piece.delay}s;
            animation-duration: {piece.duration}s;
            transform: rotate({piece.rotate}deg);
          "
        ></span>
      {/each}
    </div>

    <div class="animate-pop rounded-2xl border border-border bg-card px-8 py-7 text-center shadow-xl">
      {#if celebration.levelUp}
        <Trophy class="mx-auto mb-3 h-14 w-14 text-amber-500" />
        <p class="text-sm font-medium uppercase tracking-wider text-amber-500">Level geschafft!</p>
      {:else}
        <Sparkles class="mx-auto mb-3 h-12 w-12 text-primary" />
      {/if}

      <p class="text-4xl font-bold tabular-nums text-primary">+{celebration.points}</p>
      <p class="text-sm text-muted-foreground">Punkte</p>
      <p class="mt-2 max-w-[220px] text-sm font-medium">{celebration.label}</p>
    </div>
  </div>
{/if}

<style>
  .confetti {
    animation-name: fall;
    animation-timing-function: cubic-bezier(0.3, 0.7, 0.5, 1);
    animation-fill-mode: forwards;
    border-radius: 2px;
  }

  @keyframes fall {
    0% {
      transform: translateY(-10vh) rotate(0deg);
      opacity: 1;
    }
    100% {
      transform: translateY(105vh) rotate(720deg);
      opacity: 0;
    }
  }

  /* Respect people who get motion sick — show the badge, skip the shower. */
  @media (prefers-reduced-motion: reduce) {
    .confetti {
      display: none;
    }
  }
</style>
