/**
 * Hängt ein Element an <body>, solange es existiert.
 *
 * Fenster (Modal) werden von den Kacheln aus aufgerufen und standen deshalb
 * im Dokument INNERHALB der Kachel. Das ging gut, bis die Kachel eine
 * Weichzeichner-Fläche wurde (Oberfläche „Glas"): Ein Element mit
 * backdrop-filter wird zum Bezugsrahmen für alles, was `position: fixed` hat.
 * Das Fenster füllte dann nur noch die Kachel statt des Bildschirms, ragte
 * über die Nachbarn, und was darunter lag, liess sich weder sehen noch
 * antippen.
 *
 * Direkt unter <body> gibt es keine Kachel mehr, die das stören könnte.
 */
export function portal(node: HTMLElement) {
  document.body.appendChild(node);
  return {
    destroy() {
      node.remove();
    },
  };
}
