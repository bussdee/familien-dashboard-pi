//go:build linux

package files

import "syscall"

// freierPlatz sagt, wie viel auf dem Datenträger unter dir noch frei ist.
//
// Die Angabe steht in einem eigenen Paar von Dateien, weil Statfs nicht auf
// jedem Betriebssystem gleich aussieht. Auf dem Pi läuft Linux, die
// Entwicklung darf aber anderswo stattfinden, ohne dass sich das Backend
// nicht mehr übersetzen lässt.
//
// Bavail ist bewusst nicht Bfree: Ein Teil des Datenträgers ist für root
// reserviert, und das Backend läuft nicht als root.
func freierPlatz(dir string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0
	}
	return int64(st.Bavail) * int64(st.Bsize)
}
