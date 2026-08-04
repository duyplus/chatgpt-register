package codexreg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

func isPortStr(s string) bool {
	p, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && p > 0 && p <= 65535
}

// normalizeProxy converts formats like ip:port, host:port:user:pass, user:pass:host:port to standard URL; returns as-is if scheme exists.
func normalizeProxy(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		return raw
	}
	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 2: // ip:port or host:port
		return "http://" + parts[0] + ":" + parts[1]
	case 4: // host:port:user:pass or user:pass:host:port
		if isPortStr(parts[1]) {
			return "http://" + url.QueryEscape(parts[2]) + ":" + url.QueryEscape(parts[3]) + "@" + parts[0] + ":" + parts[1]
		} else if isPortStr(parts[3]) {
			return "http://" + url.QueryEscape(parts[0]) + ":" + url.QueryEscape(parts[1]) + "@" + parts[2] + ":" + parts[3]
		}
		return "http://" + url.QueryEscape(parts[2]) + ":" + url.QueryEscape(parts[3]) + "@" + parts[0] + ":" + parts[1]
	default:
		return "http://" + raw
	}
}

// parseProxy parses proxy string into scheme://host:port for Chrome --proxy-server (without user/pass)
// and separate username/password for browser.HandleAuth.
func parseProxy(raw string) (server, user, pass string, err error) {
	u, err := url.Parse(normalizeProxy(raw))
	if err != nil {
		return "", "", "", err
	}
	if u.Host == "" {
		return "", "", "", fmt.Errorf("proxy missing host: %s", raw)
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}
	server = scheme + "://" + u.Host
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}
	return server, user, pass, nil
}

// geoInfo geolocation result from ip-api.com.
type geoInfo struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Timezone    string  `json:"timezone"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Query       string  `json:"query"`
}

// lookupGeoIPViaRequest sends HTTP request via proxy to query exit IP geolocation.
func lookupGeoIPViaRequest(in Input) *geoInfo {
	in.logf("🌐 Querying exit IP geolocation via proxy...")

	transport := &http.Transport{}
	if strings.TrimSpace(in.Proxy) != "" {
		pu, perr := url.Parse(normalizeProxy(in.Proxy))
		if perr != nil {
			in.logf("⚠️ Proxy parsing failed, skipping geolocation alignment: %v", perr)
			return nil
		}
		transport.Proxy = http.ProxyURL(pu)
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport}

	req, err := http.NewRequest(http.MethodGet,
		"http://ip-api.com/json/?fields=status,message,country,countryCode,region,city,timezone,lat,lon,query", nil)
	if err != nil {
		in.logf("⚠️ GeoIP query failed, skipping geolocation alignment: %v", err)
		return nil
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := client.Do(req)
	if err != nil {
		in.logf("⚠️ GeoIP query failed, skipping geolocation alignment: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var g geoInfo
	if err := json.Unmarshal(body, &g); err != nil || g.Status != "success" {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		in.logf("⚠️ GeoIP query failed, skipping geolocation alignment (HTTP %d, resp=%q)", resp.StatusCode, snippet)
		return nil
	}
	in.logf("📍 Exit IP=%s Location=%s/%s Timezone=%s (%.4f, %.4f)",
		g.Query, g.CountryCode, g.City, g.Timezone, g.Lat, g.Lon)
	return &g
}

// applyGeo maps geolocation into browser: timezone, coordinates, locale, Accept-Language.
func applyGeo(page *rod.Page, g *geoInfo, in Input) {
	if g.Timezone != "" {
		_ = (proto.EmulationSetTimezoneOverride{TimezoneID: g.Timezone}).Call(page)
	}
	lat, lon, acc := g.Lat, g.Lon, 50.0
	_ = (proto.EmulationSetGeolocationOverride{Latitude: &lat, Longitude: &lon, Accuracy: &acc}).Call(page)

	locale, acceptLang := localeForCountry(g.CountryCode)
	_ = (proto.EmulationSetLocaleOverride{Locale: locale}).Call(page)
	in.logf("✅ Aligned timezone/coordinates/language: tz=%s locale=%s lang=%s", g.Timezone, locale, acceptLang)
}

// localeForCountry maps country code to ICU locale and Accept-Language, defaults to en-US.
func localeForCountry(cc string) (locale, acceptLang string) {
	switch strings.ToUpper(strings.TrimSpace(cc)) {
	case "US":
		return "en_US", "en-US,en;q=0.9"
	case "GB", "UK":
		return "en_GB", "en-GB,en;q=0.9"
	case "CA":
		return "en_CA", "en-CA,en;q=0.9,fr-CA;q=0.8"
	case "AU":
		return "en_AU", "en-AU,en;q=0.9"
	case "DE":
		return "de_DE", "de-DE,de;q=0.9,en;q=0.8"
	case "FR":
		return "fr_FR", "fr-FR,fr;q=0.9,en;q=0.8"
	case "ES":
		return "es_ES", "es-ES,es;q=0.9,en;q=0.8"
	case "IT":
		return "it_IT", "it-IT,it;q=0.9,en;q=0.8"
	case "NL":
		return "nl_NL", "nl-NL,nl;q=0.9,en;q=0.8"
	case "JP":
		return "ja_JP", "ja-JP,ja;q=0.9,en;q=0.8"
	case "KR":
		return "ko_KR", "ko-KR,ko;q=0.9,en;q=0.8"
	case "BR":
		return "pt_BR", "pt-BR,pt;q=0.9,en;q=0.8"
	case "RU":
		return "ru_RU", "ru-RU,ru;q=0.9,en;q=0.8"
	case "IN":
		return "en_IN", "en-IN,en;q=0.9,hi;q=0.8"
	case "SG":
		return "en_SG", "en-SG,en;q=0.9"
	default:
		return "en_US", "en-US,en;q=0.9"
	}
}

// blockResources blocks image/font/media requests to reduce bandwidth and detection footprint.
func blockResources(page *rod.Page, in Input) func() {
	router := page.HijackRequests()
	router.MustAdd("*", func(ctx *rod.Hijack) {
		switch ctx.Request.Type() {
		case proto.NetworkResourceTypeImage,
			proto.NetworkResourceTypeMedia,
			proto.NetworkResourceTypeFont:
			ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
		default:
			ctx.ContinueRequest(&proto.FetchContinueRequest{})
		}
	})
	go router.Run()
	in.logf("🚫 Enabled resource blocking: image/media/font")
	return router.MustStop
}
