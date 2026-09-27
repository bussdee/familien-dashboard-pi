/**
 * Kurze Rückmeldungen am unteren Rand: „Auf der Liste", „Angefragt".
 *
 * Wichtiger als die Bestätigung ist das Rückgängig. Ein versehentlich
 * gelöschter Eintrag auf der Einkaufsliste soll mit einem Tipp zurückkommen,
 * statt vorher jedes Mal mit einem Dialog nachzufragen — der hält im Laden
 * nur auf.
 */
export interface Toast {
  id: number;
  text: string;
  ton: 'info' | 'erfolg' | 'fehler';
  aktion?: { label: string; run: () => void | Promise<void> };
}

class ToastStore {
  liste = $state<Toast[]>([]);
  private naechste = 1;

  zeigen(
    text: string,
    optionen: { ton?: Toast['ton']; aktion?: Toast['aktion']; dauer?: number } = {},
  ) {
    const toast: Toast = { id: this.naechste++, text, ton: optionen.ton ?? 'info', aktion: optionen.aktion };
    // Höchstens drei auf einmal; die älteste weicht.
    this.liste = [...this.liste.slice(-2), toast];
    // Mit Rückgängig länger stehen lassen: Man muss erst merken, dass man
    // sich vertippt hat.
    const dauer = optionen.dauer ?? (toast.aktion ? 6000 : 3200);
    setTimeout(() => this.weg(toast.id), dauer);
    return toast.id;
  }

  weg(id: number) {
    this.liste = this.liste.filter((t) => t.id !== id);
  }
}

export const toasts = new ToastStore();
export const toast = (text: string, optionen?: Parameters<ToastStore['zeigen']>[1]) =>
  toasts.zeigen(text, optionen);
