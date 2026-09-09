//go:build !linux

package files

// freierPlatz kennt den freien Platz nur unter Linux. 0 heisst "unbekannt";
// die Oberfläche schweigt dann darüber, statt eine erfundene Zahl anzuzeigen.
func freierPlatz(string) int64 { return 0 }
