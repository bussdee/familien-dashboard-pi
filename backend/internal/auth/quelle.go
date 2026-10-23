package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// Woher eine Anfrage kommt.
//
// Die Sperre nach Fehlversuchen gilt je Gerät, und dafür muss der Server das
// Gerät erkennen. Hinter Traefik und nginx sieht er nur deren Adresse; die
// des Geräts steht im Kopf X-Forwarded-For. Den schreibt aber jeder mit, der
// eine Anfrage schickt — ein Kind könnte sich pro Versuch eine neue Adresse
// ausdenken und die Sperre aushebeln.
//
// Verlass ist nur auf das, was die eigenen Zwischenstationen angehängt haben.
// Jede hängt an die Liste an, was sie als Absender gesehen hat, und zwar
// rechts. Beim Dashboard sind das zwei: Traefik hängt das Gerät an, nginx
// hängt Traefik an, und Traefik selbst ist die Verbindung, die das Backend
// sieht. Die Kette lautet damit
//
//	[ was das Gerät behauptet …, Gerät, Traefik, nginx (= RemoteAddr) ]
//
// und das Gerät steht zwei Stellen vor dem Ende. Alles links davon ist
// ungeprüft und wird nicht gelesen.
//
// middleware.RealIP aus chi nimmt dagegen den ersten Eintrag (und glaubt
// True-Client-IP und X-Real-IP): Genau der ist frei erfunden.

// StandardProxyStufen: Traefik und nginx.
const StandardProxyStufen = 2

// ClientAdresse bestimmt die Adresse des Geräts. stufen ist die Zahl der
// eigenen Zwischenstationen vor dem Backend.
func ClientAdresse(r *http.Request, stufen int) string {
	direkt := hostVon(r.RemoteAddr)

	var kette []string
	for _, zeile := range r.Header.Values("X-Forwarded-For") {
		for _, teil := range strings.Split(zeile, ",") {
			kette = append(kette, strings.TrimSpace(teil))
		}
	}
	kette = append(kette, direkt)

	if stufen < 0 {
		stufen = 0
	}
	i := len(kette) - 1 - stufen
	if i < 0 {
		// Weniger Stationen als erwartet: Dann ist nichts davon belegt, und
		// es bleibt die direkte Verbindung — die lässt sich nicht erfinden.
		i = len(kette) - 1
	}

	if ip, err := netip.ParseAddr(kette[i]); err == nil {
		return normalisiert(ip)
	}
	return normalisiert(parseOder(direkt))
}

// RealClientIP ersetzt chi's RealIP: Es setzt RemoteAddr auf die Adresse des
// Geräts, so wie ClientAdresse sie bestimmt, und nicht auf eine, die der
// Absender selbst behauptet hat.
func RealClientIP(stufen int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.RemoteAddr = net.JoinHostPort(ClientAdresse(r, stufen), "0")
			next.ServeHTTP(w, r)
		})
	}
}

// Client sagt, als welches Gerät der Server diese Anfrage sieht. Nur für
// Administratoren, zum Nachprüfen von server.proxy_hops: Steht hier die
// Adresse von Traefik oder nginx (172.x) statt der des eigenen Geräts, ist die
// Zahl falsch, und alle Geräte teilen sich eine Sperre.
func (s *Service) Client(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"address": quelle(r)})
}

// quelle ist der Schlüssel, unter dem Fehlversuche eines Geräts gezählt
// werden. Nach RealClientIP steht die Adresse des Geräts in RemoteAddr.
func quelle(r *http.Request) string {
	return normalisiert(parseOder(hostVon(r.RemoteAddr)))
}

func hostVon(adresse string) string {
	if host, _, err := net.SplitHostPort(adresse); err == nil {
		return host
	}
	return strings.TrimSpace(adresse)
}

func parseOder(s string) netip.Addr {
	ip, err := netip.ParseAddr(strings.TrimSpace(s))
	if err != nil {
		return netip.Addr{}
	}
	return ip
}

// normalisiert macht aus der Adresse einen stabilen Schlüssel. IPv4-in-IPv6
// wird zu IPv4. Ein IPv6-Gerät wechselt seine Adresse laufend (Privacy
// Extensions), bleibt aber im selben /64-Netz — das ist die Einheit, die
// zählt, sonst wäre die Sperre mit jeder neuen Adresse umgangen.
func normalisiert(ip netip.Addr) string {
	if !ip.IsValid() {
		return "unbekannt"
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is6() {
		if p, err := ip.Prefix(64); err == nil {
			return p.String()
		}
	}
	return ip.String()
}
