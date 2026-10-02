<script lang="ts">
  import { t } from '$lib/i18n';
  import { page } from '$app/stores';
  import { Menu, Plus, ShoppingCart, House, UtensilsCrossed } from 'lucide-svelte';
  import { aktiv, menu } from '$lib/stores/navigation.svelte';
  import { schnell } from '$lib/stores/schnell.svelte';

  /**
   * Die Leiste am unteren Rand, auf Handy und Tablet im Hochformat.
   *
   * Bis 1.7 gab es auf dem Handy nur den Menüknopf oben links — also genau
   * dort, wo der Daumen nicht hinkommt. Im Laden, mit dem Korb in der
   * anderen Hand, ist das die falsche Ecke. Jetzt liegen die drei häufigsten
   * Ziele und das Plus unten, wo der Daumen ohnehin ist.
   */
  const pfad = $derived($page.url.pathname);

  const links = [
    { href: '/', label: t('Start'), icon: House },
    { href: '/einkaufen', label: t('Einkauf'), icon: ShoppingCart },
  ];
  const rechts = [{ href: '/essen', label: t('Essen'), icon: UtensilsCrossed }];
</script>

{#snippet ziel(item: { href: string; label: string; icon: typeof House })}
  {@const an = aktiv(pfad, item.href)}
  <a
    href={item.href}
    class="flex flex-1 flex-col items-center justify-center gap-0.5 py-1.5 text-[11px] font-medium transition-colors
      {an ? 'text-primary' : 'text-muted-foreground'}"
    aria-current={an ? 'page' : undefined}
  >
    <span class="flex h-8 w-14 items-center justify-center rounded-full transition-colors {an ? 'bg-primary/15' : ''}">
      <item.icon class="h-5 w-5" />
    </span>
    {item.label}
  </a>
{/snippet}

<nav
  class="safe-bottom fixed inset-x-0 bottom-0 z-40 border-t border-[color:var(--haarlinie-stark)] bg-background/85 backdrop-blur-xl lg:hidden"
  aria-label={t('Schnellnavigation')}
>
  <div class="mx-auto flex h-[4.25rem] max-w-lg items-stretch px-1">
    {#each links as item (item.href)}
      {@render ziel(item)}
    {/each}

    <!-- Das Plus in der Mitte: der eine Knopf, den man blind trifft. -->
    <div class="flex flex-1 items-center justify-center">
      <button
        class="flex h-14 w-14 -translate-y-3 items-center justify-center rounded-2xl bg-primary text-primary-foreground shadow-lg shadow-[rgb(var(--akzent-rgb)/0.35)] transition-transform active:scale-95"
        onclick={() => (schnell.offen = true)}
        aria-label={t('Schnell hinzufügen')}
        aria-haspopup="dialog"
      >
        <Plus class="h-7 w-7" />
      </button>
    </div>

    {#each rechts as item (item.href)}
      {@render ziel(item)}
    {/each}

    <button
      class="flex flex-1 flex-col items-center justify-center gap-0.5 py-1.5 text-[11px] font-medium text-muted-foreground"
      onclick={() => (menu.offen = true)}
      aria-label={t('Menü öffnen')}
      aria-expanded={menu.offen}
    >
      <span class="flex h-8 w-14 items-center justify-center rounded-full">
        <Menu class="h-5 w-5" />
      </span>
      {t('Mehr')}
    </button>
  </div>
</nav>
