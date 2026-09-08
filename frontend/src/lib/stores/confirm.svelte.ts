/**
 * In-app confirmation dialogs.
 *
 * The app used to call window.confirm(). That is unreliable: several browsers
 * suppress native dialogs outright (they then return false, so the action
 * silently does nothing), and inside an installed PWA a system dialog looks
 * out of place. This store hands out a promise instead, resolved by a real
 * component that matches the rest of the interface.
 */
export interface ConfirmRequest {
  title: string;
  message?: string;
  /** Label of the confirming button. */
  confirmLabel?: string;
  cancelLabel?: string;
  /** Renders the confirming button in the destructive colour. */
  danger?: boolean;
}

class ConfirmStore {
  request = $state<ConfirmRequest | null>(null);
  private resolver: ((value: boolean) => void) | null = null;

  ask(options: ConfirmRequest): Promise<boolean> {
    // A second request while one is open cancels the first rather than
    // stacking dialogs on top of each other.
    this.resolver?.(false);

    this.request = {
      confirmLabel: 'Löschen',
      cancelLabel: 'Abbrechen',
      danger: true,
      ...options,
    };

    return new Promise<boolean>((resolve) => {
      this.resolver = resolve;
    });
  }

  answer(value: boolean) {
    this.request = null;
    const resolve = this.resolver;
    this.resolver = null;
    resolve?.(value);
  }
}

export const confirmStore = new ConfirmStore();

/** Shorthand: `if (!(await confirmAction({ title: '…' }))) return;` */
export const confirmAction = (options: ConfirmRequest) => confirmStore.ask(options);
