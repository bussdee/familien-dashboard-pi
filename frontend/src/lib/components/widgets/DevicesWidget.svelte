<script lang="ts">
  import {
    Activity, ExternalLink, HardDrive, RefreshCw, Server, Wifi, WifiOff,
  } from 'lucide-svelte';
  import { format, parseISO } from 'date-fns';
  import type { DeviceStatus } from '$lib/types';

  let {
    devices = [],
    onRefresh,
  }: { devices?: DeviceStatus[]; onRefresh: () => Promise<void> } = $props();

  let refreshing = $state(false);

  const icons: Record<string, typeof Server> = {
    Plex: Server,
    Kavita: HardDrive,
    FileBrowser: HardDrive,
  };

  const online = $derived(devices.filter((d) => d.status === 'up').length);

  // A status maps to a real colour, not a Tailwind class name — these values
  // are used in inline styles.
  const colors: Record<string, string> = {
    up: '#22c55e',
    down: '#ef4444',
    unknown: '#94a3b8',
  };
  const labels: Record<string, string> = { up: 'Online', down: 'Offline', unknown: 'Unbekannt' };

  async function refresh() {
    refreshing = true;
    try {
      await onRefresh();
    } finally {
      refreshing = false;
    }
  }
</script>

<section class="card p-5">
  <header class="mb-4 flex items-center justify-between">
    <div>
      <h2 class="flex items-center gap-2 text-lg font-semibold">
        <Wifi class="h-5 w-5" /> Geräte
      </h2>
      <p class="text-sm text-muted-foreground">
        {online} von {devices.length} erreichbar
      </p>
    </div>
    <button class="btn-ghost px-2" onclick={refresh} aria-label="Status aktualisieren">
      <RefreshCw class="h-5 w-5 {refreshing ? 'animate-spin' : ''}" />
    </button>
  </header>

  {#if devices.length === 0}
    <div class="py-8 text-center text-muted-foreground">
      <WifiOff class="mx-auto mb-2 h-10 w-10 opacity-40" />
      <p class="text-sm">Keine Geräte konfiguriert</p>
      <p class="mt-1 text-xs">Siehe <code>backend/config.yaml</code></p>
    </div>
  {:else}
    <ul class="space-y-2">
      {#each devices as device (device.name)}
        {@const Icon = icons[device.name] ?? Activity}
        {@const color = colors[device.status] ?? colors.unknown}
        {@const reachable = device.status === 'up' && !!device.link}
        <li>
          <!--
            A reachable device with a web UI becomes a link; an unreachable one
            stays a plain tile, so nobody taps into a connection error.
          -->
          <svelte:element
            this={reachable ? 'a' : 'div'}
            href={reachable ? device.link : undefined}
            target={reachable ? '_blank' : undefined}
            rel={reachable ? 'noopener noreferrer' : undefined}
            class="group flex items-center gap-3 rounded-lg bg-muted/30 p-3 transition-colors
              {reachable ? 'hover:bg-accent' : ''}"
          >
            <div
              class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg"
              style="background-color: {color}1a"
            >
              <Icon class="h-5 w-5" style="color: {color}" />
            </div>

            <div class="min-w-0 flex-1">
              <p class="flex items-center gap-1.5 truncate text-sm font-medium">
                {device.name}
                {#if reachable}
                  <ExternalLink
                    class="h-3 w-3 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100"
                  />
                {/if}
              </p>
              <p class="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span class="h-2 w-2 shrink-0 rounded-full" style="background-color: {color}"></span>
                {labels[device.status] ?? device.status}
                {#if device.status === 'up' && device.latency_ms > 0}
                  · {device.latency_ms} ms
                {/if}
              </p>
            </div>

            <div class="shrink-0 text-right">
              {#if device.last_check}
                <p class="text-xs text-muted-foreground">
                  {format(parseISO(device.last_check), 'HH:mm')}
                </p>
              {/if}
              {#if device.error}
                <p class="max-w-[150px] truncate text-[11px] text-destructive" title={device.error}>
                  {device.error}
                </p>
              {/if}
            </div>
          </svelte:element>
        </li>
      {/each}
    </ul>
    <p class="mt-3 text-xs text-muted-foreground">
      Antippen öffnet die Oberfläche des Geräts in einem neuen Tab.
    </p>
  {/if}
</section>
