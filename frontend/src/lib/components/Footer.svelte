<script lang="ts">
  import { Heart } from 'lucide-svelte';
</script>

<!--
  Bewusst zurückhaltend: eine Zeile am Seitenende, die im Alltag nicht stört,
  aber auffindbar ist. Auf dem Handy sitzt sie über der Menüzeile.
-->
<footer class="mt-8 border-t border-border/60 px-4 py-5 text-center">
  <p class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
    <span>Familien Dashboard</span>
    <span aria-hidden="true">·</span>
    <a
      href="https://familienfabrik.at"
      target="_blank"
      rel="noopener noreferrer"
      class="transition-colors hover:text-foreground hover:underline"
    >
      familienfabrik.at
    </a>
    <span aria-hidden="true">·</span>
    <a
      href="https://paypal.me/bussdee"
      target="_blank"
      rel="noopener noreferrer"
      class="inline-flex items-center gap-1 transition-colors hover:text-foreground hover:underline"
    >
      <Heart class="h-3 w-3" />
      Spenden
    </a>
  </p>
</footer>
