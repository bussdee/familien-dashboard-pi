<script lang="ts">
  import type { Score } from '$lib/types';

  let {
    score,
    showLabel = true,
    size = 'md',
  }: { score: Score; showLabel?: boolean; size?: 'sm' | 'md' } = $props();
</script>

<div class="w-full">
  {#if showLabel}
    <div class="mb-1 flex items-baseline justify-between gap-2 text-xs">
      <span class="font-medium">Level {score.level} · {score.level_name}</span>
      <span class="text-muted-foreground tabular-nums">
        noch {score.points_to_next} P
      </span>
    </div>
  {/if}
  <div
    class="overflow-hidden rounded-full bg-muted {size === 'sm' ? 'h-1.5' : 'h-2.5'}"
    role="progressbar"
    aria-valuenow={score.level_progress}
    aria-valuemin={0}
    aria-valuemax={100}
    aria-label="Fortschritt zu Level {score.level + 1}"
  >
    <div
      class="h-full rounded-full bg-gradient-to-r from-primary to-emerald-400 transition-[width] duration-700"
      style="width: {score.level_progress}%"
    ></div>
  </div>
</div>
