<script lang="ts">
  import {
    Cloud, CloudDrizzle, CloudFog, CloudLightning, CloudRain, CloudSnow, CloudSun, Sun,
  } from 'lucide-svelte';
  import type { WeatherData } from '$lib/types';

  let { weather }: { weather: WeatherData | null } = $props();

  const icons: Record<string, typeof Cloud> = {
    sun: Sun,
    'cloud-sun': CloudSun,
    cloud: Cloud,
    'cloud-drizzle': CloudDrizzle,
    'cloud-rain': CloudRain,
    'cloud-snow': CloudSnow,
    'cloud-fog': CloudFog,
    'cloud-lightning': CloudLightning,
  };

  const Icon = $derived(icons[weather?.current.icon ?? ''] ?? Cloud);

  // Sun in daylight is amber; everything else keeps the calmer accent colour.
  const tone = $derived(
    weather?.current.icon === 'sun' && weather.current.is_day
      ? 'text-amber-500'
      : weather?.current.icon === 'cloud-lightning'
        ? 'text-violet-500'
        : 'text-primary',
  );

  const rainToday = $derived(weather?.forecast[0]?.precip_probability ?? 0);
</script>

{#if weather}
  <a
    href="/wetter"
    class="flex items-center gap-2 rounded-xl px-2 py-1 transition-colors hover:bg-accent"
    title="{weather.current.description} in {weather.location.name} – Details ansehen"
  >
    <Icon class="h-7 w-7 shrink-0 {tone}" />
    <span class="leading-tight">
      <span class="block text-lg font-semibold tabular-nums">
        {Math.round(weather.current.temperature)}°
      </span>
      <span class="block text-[11px] text-muted-foreground">
        {#if rainToday > 30}
          {rainToday}% Regen
        {:else}
          {weather.current.description}
        {/if}
      </span>
    </span>
  </a>
{/if}
