package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"warofages/internal/db"
	"warofages/internal/util"
)

const adminPass = "admin-password-1"

// site is a running server with the real router and a temporary database.
type site struct {
	t   *testing.T
	srv *httptest.Server
}

func newSite(t *testing.T) *site {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("../.."); err != nil { // templates load relative to the repo root
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	h, err := buildHandler(util.Config{AdminUsername: "gamemaster", AdminPassword: adminPass}, conn)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &site{t, srv}
}

// browser is one logged-in user with a cookie jar that doesn't follow redirects.
type browser struct {
	t      *testing.T
	base   string
	client *http.Client
	header map[string]string
}

func (s *site) browser() *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{t: s.t, base: s.srv.URL, client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (b *browser) do(method, path string, form url.Values) (*http.Response, string) {
	b.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, b.base+path, body)
	if err != nil {
		b.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range b.header {
		req.Header.Set(k, v)
	}
	res, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res, string(raw)
}

func (b *browser) get(path string) (*http.Response, string) { return b.do(http.MethodGet, path, nil) }

func (b *browser) post(path string, kv ...string) (*http.Response, string) {
	form := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		form.Add(kv[i], kv[i+1])
	}
	return b.do(http.MethodPost, path, form)
}

func (b *browser) login(user, pass string) *http.Response {
	b.t.Helper()
	res, _ := b.post("/login", "username", user, "password", pass, "next", "/sheets")
	return res
}

func wantStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		t.Fatalf("%s %s: status %d, want %d", res.Request.Method, res.Request.URL.Path, res.StatusCode, want)
	}
}

func wantRedirect(t *testing.T, res *http.Response, prefix string) {
	t.Helper()
	wantStatus(t, res, http.StatusSeeOther)
	if loc := res.Header.Get("Location"); !strings.HasPrefix(loc, prefix) {
		t.Fatalf("redirected to %q, want prefix %q", loc, prefix)
	}
}

// admin logs in as the bootstrap admin and clears the forced password change.
func (s *site) admin() *browser {
	s.t.Helper()
	b := s.browser()
	wantRedirect(s.t, b.login("gamemaster", adminPass), "/sheets")
	res, _ := b.post("/account", "current_password", adminPass, "new_password", "admin-password-2", "confirm_password", "admin-password-2")
	wantRedirect(s.t, res, "/account")
	return b
}

// player has the admin create an account and logs in, ready to play.
func (s *site) player(admin *browser, name string) (*browser, string) {
	s.t.Helper()
	res, _ := admin.post("/admin/players", "username", name, "password", "temporary-pass", "role", "player")
	wantStatus(s.t, res, http.StatusOK)
	b := s.browser()
	wantRedirect(s.t, b.login(name, "temporary-pass"), "/sheets")
	res, _ = b.post("/account", "current_password", "temporary-pass", "new_password", "player-pass-1", "confirm_password", "player-pass-1")
	wantRedirect(s.t, res, "/account")
	return b, "player-pass-1"
}

var sheetPath = regexp.MustCompile(`^/sheets/(\d+)$`)

func (b *browser) createCharacter(name string) string {
	b.t.Helper()
	res, body := b.post("/sheets/new",
		"name", name, "race", "human", "class", "wizard", "level", "3", "background", "acolyte", "alignment", "True Neutral",
		"method", "standard", "score_str", "8", "score_dex", "14", "score_con", "13", "score_int", "15", "score_wis", "12", "score_cha", "10",
		"skills", "arcana", "skills", "history")
	if res.StatusCode != http.StatusSeeOther {
		b.t.Fatalf("create character: status %d\n%s", res.StatusCode, body)
	}
	loc := res.Header.Get("Location")
	if !sheetPath.MatchString(loc) {
		b.t.Fatalf("unexpected location %q", loc)
	}
	return loc
}

func TestLoggedOutIsSentToLogin(t *testing.T) {
	s := newSite(t)
	b := s.browser()
	for _, path := range []string{"/sheets", "/sheets/new", "/sheets/1", "/admin", "/account", "/sheets/compendium/spells"} {
		res, _ := b.get(path)
		wantRedirect(t, res, "/login")
	}
	res, _ := b.post("/sheets/1/hp", "amount", "1", "op", "damage")
	wantRedirect(t, res, "/login")
}

func TestLoginPageIsReachableAndThereIsNoSignup(t *testing.T) {
	s := newSite(t)
	b := s.browser()
	res, body := b.get("/login")
	wantStatus(t, res, http.StatusOK)
	if strings.Contains(strings.ToLower(body), "sign up") || strings.Contains(strings.ToLower(body), "register") {
		t.Error("login page offers registration")
	}
	res, _ = b.post("/register", "username", "mallory", "password", "password123")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("/register status %d", res.StatusCode)
	}
}

func TestLoginErrorsAreTheSameForUnknownUsers(t *testing.T) {
	s := newSite(t)
	_, wrongPass := s.browser().post("/login", "username", "gamemaster", "password", "wrong-password")
	_, unknown := s.browser().post("/login", "username", "nobody", "password", "wrong-password")
	strip := regexp.MustCompile(`value="[^"]*"`)
	if strip.ReplaceAllString(wrongPass, "") != strip.ReplaceAllString(unknown, "") {
		t.Error("login failure reveals whether a username exists")
	}
}

func TestLoginThrottle(t *testing.T) {
	s := newSite(t)
	b := s.browser()
	for i := 0; i < 5; i++ {
		res, _ := b.post("/login", "username", "gamemaster", "password", "wrong-password")
		wantStatus(t, res, http.StatusUnauthorized)
	}
	res, _ := b.post("/login", "username", "gamemaster", "password", adminPass)
	wantStatus(t, res, http.StatusTooManyRequests)
}

func TestSafeNext(t *testing.T) {
	s := newSite(t)
	for _, next := range []string{"https://evil.example/", "//evil.example/", "/\\evil.example"} {
		res, _ := s.browser().post("/login", "username", "gamemaster", "password", adminPass, "next", next)
		wantRedirect(t, res, "/sheets")
	}
}

func TestForcedPasswordChange(t *testing.T) {
	s := newSite(t)
	b := s.browser()
	wantRedirect(t, b.login("gamemaster", adminPass), "/sheets")
	for _, path := range []string{"/sheets", "/admin", "/sheets/new"} {
		res, _ := b.get(path)
		wantRedirect(t, res, "/account")
	}
	res, _ := b.post("/account", "current_password", "wrong", "new_password", "a-new-password", "confirm_password", "a-new-password")
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = b.post("/account", "current_password", adminPass, "new_password", "short", "confirm_password", "short")
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = b.post("/account", "current_password", adminPass, "new_password", adminPass, "confirm_password", adminPass)
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = b.post("/account", "current_password", adminPass, "new_password", "a-new-password", "confirm_password", "a-new-password")
	wantRedirect(t, res, "/account")
	res, _ = b.get("/sheets")
	wantStatus(t, res, http.StatusOK)

	// The old password no longer works.
	if res := s.browser().login("gamemaster", adminPass); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("old password status %d", res.StatusCode)
	}
}

func TestPasswordsAreStoredAsBcryptAndTokensHashed(t *testing.T) {
	wd, _ := os.Getwd()
	os.Chdir("../..")
	t.Cleanup(func() { os.Chdir(wd) })
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	h, err := buildHandler(util.Config{AdminUsername: "gamemaster", AdminPassword: adminPass}, conn)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	b := (&site{t, srv}).browser()
	res := b.login("gamemaster", adminPass)

	var hash string
	if err := conn.QueryRow(`SELECT password_hash FROM users WHERE username = 'gamemaster'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$2a$12$") {
		t.Errorf("hash is not bcrypt cost 12: %.7s", hash)
	}
	var cookie string
	for _, c := range res.Cookies() {
		if c.Name == "woa_session" {
			cookie = c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Errorf("cookie flags: %+v", c)
			}
		}
	}
	var stored string
	if err := conn.QueryRow(`SELECT token_hash FROM sessions`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if cookie == "" || stored == cookie {
		t.Error("session token stored in plaintext")
	}
}

func TestPlayerCantReachAdmin(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	for _, path := range []string{"/admin", "/admin/players", "/admin/players/1"} {
		res, _ := p.get(path)
		wantStatus(t, res, http.StatusNotFound)
	}
	res, _ := p.post("/admin/players", "username", "mallory", "password", "password123", "role", "admin")
	wantStatus(t, res, http.StatusNotFound)
	res, _ = p.post("/admin/players/1/delete", "confirm", "yes")
	wantStatus(t, res, http.StatusNotFound)
}

func TestPlayersCantReachEachOthersSheets(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	alice, _ := s.player(admin, "alice")
	bobby, _ := s.player(admin, "bobby")
	sheet := alice.createCharacter("Aster")

	for _, path := range []string{sheet, sheet + "/edit", sheet + "/levelup"} {
		res, _ := bobby.get(path)
		wantStatus(t, res, http.StatusNotFound)
	}
	id := strings.TrimPrefix(sheet, "/sheets/")
	res, _ := bobby.get("/sheets/compendium/spells?sheet=" + id + "&spell_q=fire")
	wantStatus(t, res, http.StatusNotFound)
	for _, path := range []string{"/hp", "/delete", "/text", "/edit", "/levelup", "/roll"} {
		res, _ := bobby.post(sheet+path, "amount", "5", "op", "damage", "confirm", "yes", "notes", "hacked")
		wantStatus(t, res, http.StatusNotFound)
	}
	res, body := alice.get(sheet)
	wantStatus(t, res, http.StatusOK)
	if strings.Contains(body, "hacked") {
		t.Error("another player's write landed")
	}

	// The admin can see and edit it.
	res, body = admin.get(sheet)
	wantStatus(t, res, http.StatusOK)
	if !strings.Contains(body, "viewing alice") {
		t.Error("admin banner missing")
	}
}

func TestCrossOriginPostsAreRejected(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	sheet := p.createCharacter("Aster")

	p.header = map[string]string{"Sec-Fetch-Site": "cross-site"}
	res, _ := p.post(sheet+"/hp", "amount", "5", "op", "damage")
	wantStatus(t, res, http.StatusForbidden)

	p.header = map[string]string{"Origin": "https://evil.example"}
	res, _ = p.post(sheet+"/hp", "amount", "5", "op", "damage")
	wantStatus(t, res, http.StatusForbidden)

	p.header = map[string]string{"Sec-Fetch-Site": "same-origin"}
	res, _ = p.post(sheet+"/hp", "amount", "5", "op", "damage")
	wantRedirect(t, res, sheet)
}

func TestSessionsAreRevoked(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")

	other := s.browser()
	wantRedirect(t, other.login("alice", "player-pass-1"), "/sheets")
	res, _ := other.get("/sheets")
	wantStatus(t, res, http.StatusOK)

	// Changing your password ends other sessions but keeps this one.
	res, _ = p.post("/account", "current_password", "player-pass-1", "new_password", "player-pass-2", "confirm_password", "player-pass-2")
	wantRedirect(t, res, "/account")
	res, _ = other.get("/sheets")
	wantRedirect(t, res, "/login")
	res, _ = p.get("/sheets")
	wantStatus(t, res, http.StatusOK)

	// Disabling ends them all and blocks login.
	res, _ = admin.post("/admin/players/2/disable")
	wantRedirect(t, res, "/admin/players/2")
	res, _ = p.get("/sheets")
	wantRedirect(t, res, "/login")
	if res := s.browser().login("alice", "player-pass-2"); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("disabled login status %d", res.StatusCode)
	}

	// Enabling restores login, and a reset forces a change again.
	res, _ = admin.post("/admin/players/2/enable")
	wantRedirect(t, res, "/admin/players/2")
	res, body := admin.post("/admin/players/2/reset")
	wantStatus(t, res, http.StatusOK)
	m := regexp.MustCompile(`<code>([^<]+)</code>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("generated password not shown")
	}
	back := s.browser()
	wantRedirect(t, back.login("alice", m[1]), "/sheets")
	res, _ = back.get("/sheets")
	wantRedirect(t, res, "/account")

	// Logging out ends the session on the server too.
	res, _ = p.post("/logout")
	wantRedirect(t, res, "/login")
	res, _ = p.get("/sheets")
	wantRedirect(t, res, "/login")
}

func TestLastAdminIsProtected(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	res, _ := admin.post("/admin/players/1/delete", "confirm", "yes")
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = admin.post("/admin/players/1/disable")
	wantStatus(t, res, http.StatusBadRequest)
}

func TestDeletingAPlayerRemovesTheirCharacters(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	sheet := p.createCharacter("Aster")
	res, _ := admin.post("/admin/players/2/delete")
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = admin.post("/admin/players/2/delete", "confirm", "yes")
	wantRedirect(t, res, "/admin")
	res, _ = admin.get(sheet)
	wantStatus(t, res, http.StatusNotFound)
}

func TestPlayerInputIsEscaped(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	const xss = `<script>alert(1)</script>`
	sheet := p.createCharacter(xss)
	p.post(sheet+"/text", "notes", xss, "backstory", xss)
	p.post(sheet+"/item/custom", "name", xss, "kind", "gear", "description", xss)
	p.post(sheet+"/feature/add", "name", xss, "description", xss)
	p.post(sheet+"/spell/custom", "name", xss, "level", "0", "description", xss)
	p.post(sheet+"/roll", "what", "custom", "expr", "1d4", "label", xss)

	pages := []*browser{p, admin}
	for _, b := range pages {
		for _, path := range []string{sheet, sheet + "/edit", sheet + "/levelup"} {
			res, body := b.get(path)
			wantStatus(t, res, http.StatusOK)
			if strings.Contains(body, xss) {
				t.Errorf("%s contains unescaped input", path)
			}
			if !strings.Contains(body, "&lt;script&gt;") {
				t.Errorf("%s doesn't show the escaped input", path)
			}
		}
	}
	_, list := p.get("/sheets")
	_, dash := admin.get("/admin")
	if strings.Contains(list, xss) || strings.Contains(dash, xss) {
		t.Error("list pages contain unescaped input")
	}
}

func TestSecurityHeaders(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	sheet := p.createCharacter("Aster")
	for _, path := range []string{"/sheets", sheet, "/account"} {
		res, _ := p.get(path)
		csp := res.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe") {
			t.Errorf("%s CSP = %q", path, csp)
		}
		if res.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s Cache-Control = %q", path, res.Header.Get("Cache-Control"))
		}
	}
	_, body := p.get(sheet)
	if regexp.MustCompile(`<script>`).MatchString(body) || regexp.MustCompile(` on[a-z]+="`).MatchString(body) {
		t.Error("inline script would be blocked by the CSP")
	}
}

func wantContains(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("response is missing %q", want)
		}
	}
}

// TestFullSession plays through a character without any client JS.
func TestFullSession(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")

	// Builder pages render, and rolling keeps what was typed.
	res, body := p.get("/sheets/new")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "Create character", "Hill Dwarf", "Berserker")
	res, body = p.post("/sheets/new/roll", "name", "Rolled", "race", "human", "class", "fighter", "level", "1", "background", "acolyte", "method", "standard")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, `value="Rolled"`, `value="manual" checked`)

	// Bad scores don't save and keep the form.
	res, body = p.post("/sheets/new", "name", "Cheater", "race", "human", "class", "fighter", "level", "1", "background", "acolyte",
		"method", "point", "score_str", "15", "score_dex", "15", "score_con", "15", "score_int", "15", "score_wis", "15", "score_cha", "15")
	wantStatus(t, res, http.StatusBadRequest)
	wantContains(t, body, "over budget", `value="Cheater"`)

	// Point buy preview comes from the server.
	res, body = p.post("/sheets/new/preview", "race", "human", "class", "fighter", "level", "1", "background", "acolyte", "method", "point",
		"score_str", "15", "score_dex", "14", "score_con", "13", "score_int", "8", "score_wis", "8", "score_cha", "8")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "<strong>21</strong> of 27 points")
	if strings.Contains(body, "<html") {
		t.Error("preview should be a fragment")
	}

	sheet := p.createCharacter("Aster")

	// Level 3 human wizard: 15+1 int, 13+1 con (+2), HP 6 + 2*4 + 3*2 = 20.
	res, body = p.get(sheet)
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "20 / 20", "Human, Wizard 3", "Spellcasting: Wizard", "Arcane Recovery")

	// Damage, then temp HP, then healing.
	res, _ = p.post(sheet+"/hp", "tab", "actions", "amount", "5", "op", "damage")
	wantRedirect(t, res, sheet+"?tab=actions")
	_, body = p.get(sheet)
	wantContains(t, body, "15 / 20")
	p.post(sheet+"/hp", "amount", "4", "op", "temp")
	p.post(sheet+"/hp", "amount", "2", "op", "damage")
	_, body = p.get(sheet)
	wantContains(t, body, "15 / 20", "2 temp")

	// Spells: search works without JS, and adding one lets you cast it.
	res, body = p.get(sheet + "?tab=spells&spell_q=magic+missile")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "Magic Missile", "1 spells found")
	res, _ = p.post(sheet+"/spell/add", "tab", "spells", "srd", "magic-missile")
	wantRedirect(t, res, sheet+"?tab=spells")
	p.post(sheet+"/spell/add", "tab", "spells", "srd", "fire-bolt")
	_, body = p.get(sheet + "?tab=spells")
	wantContains(t, body, "1st level: <strong>4</strong> of 4", "Prepare")
	spellID := regexp.MustCompile(`name="spell" value="(\d+)"`).FindStringSubmatch(body)
	if spellID == nil {
		t.Fatal("no spell form")
	}

	// Find Magic Missile's form rather than assuming an order.
	mm := regexp.MustCompile(`(?s)Magic Missile.*?name="spell" value="(\d+)"`).FindStringSubmatch(body)
	if mm == nil {
		t.Fatal("no Magic Missile form")
	}
	res, _ = p.post(sheet+"/spell/cast", "tab", "spells", "spell", mm[1], "slot", "2", "op", "cast")
	wantRedirect(t, res, sheet)
	_, body = p.get(sheet + "?tab=spells")
	wantContains(t, body, "2nd level: <strong>1</strong> of 2", "Cast Magic Missile", "Level 2 slot")
	res, _ = p.post(sheet+"/spell/cast", "spell", mm[1], "slot", "2", "op", "cast")
	wantRedirect(t, res, sheet)
	res, body = p.post(sheet+"/spell/cast", "spell", mm[1], "slot", "2", "op", "cast")
	wantStatus(t, res, http.StatusBadRequest)
	wantContains(t, body, "no spell slot")

	// Rolls name what to roll. The total comes from the server.
	res, _ = p.post(sheet+"/roll", "tab", "rolls", "what", "skill:arcana", "mode", "adv")
	wantRedirect(t, res, sheet)
	_, body = p.get(sheet + "?tab=rolls")
	wantContains(t, body, "Arcana", `id="last-roll"`, "kept")
	res, _ = p.post(sheet+"/roll", "what", "custom", "expr", "2d6+3", "label", "Test")
	wantRedirect(t, res, sheet)
	res, body = p.post(sheet+"/roll", "what", "custom", "expr", "nonsense")
	wantStatus(t, res, http.StatusBadRequest)
	wantContains(t, body, "can&#39;t read dice")
	res, _ = p.post(sheet+"/roll", "what", "skill:nope", "total", "20")
	wantStatus(t, res, http.StatusBadRequest)

	// Inventory: equip armor and see the AC change, and weigh it.
	res, _ = p.post(sheet+"/item/add", "tab", "inventory", "srd", "leather-armor")
	wantRedirect(t, res, sheet)
	p.post(sheet+"/item/add", "tab", "inventory", "srd", "dagger", "qty", "2")
	res, body = p.get(sheet + "?tab=inventory")
	wantContains(t, body, "Leather Armor", "Dagger")
	armor := regexp.MustCompile(`(?s)Leather Armor.*?name="item" value="(\d+)"`).FindStringSubmatch(body)
	if armor == nil {
		t.Fatal("no armor form")
	}
	p.post(sheet+"/item/equip", "item", armor[1])
	_, body = p.get(sheet)
	// 11 + Dex 14 (+2) = 13
	wantContains(t, body, `Leather Armor 11 &#43; Dex &#43;2`, `<strong>13</strong>`)

	// Rests: short keeps slots, long restores everything.
	p.post(sheet+"/rest", "kind", "short")
	_, body = p.get(sheet + "?tab=spells")
	wantContains(t, body, "2nd level: <strong>0</strong> of 2")
	p.post(sheet+"/rest", "kind", "long")
	_, body = p.get(sheet + "?tab=spells")
	wantContains(t, body, "2nd level: <strong>2</strong> of 2", "20 / 20")

	// Level up. Strength 9 blocks multiclassing into barbarian. Wizard always works.
	res, body = p.get(sheet + "/levelup")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "Fighter 1", "Wizard 4", "ability score improvement")
	res, body = p.post(sheet+"/levelup", "class", "barbarian", "hp_method", "average")
	wantStatus(t, res, http.StatusBadRequest)
	wantContains(t, body, "requires")
	res, _ = p.post(sheet+"/levelup", "class", "wizard", "hp_method", "average", "subclass_wizard", "evocation", "asi_1", "int", "asi_2", "con")
	wantRedirect(t, res, sheet)
	_, body = p.get(sheet + "?tab=features")
	// Con 14 to 15 keeps +2; 6 + 3*4 + 4*2 = 26 max HP with current HP raised to match.
	wantContains(t, body, "Wizard 4 (Evocation)", "26 / 26", "Evocation Savant", "Ability Score Improvement")

	// Edit: add a custom bonus and rename.
	res, _ = p.get(sheet + "/edit")
	wantStatus(t, res, http.StatusOK)

	// Custom content and notes.
	p.post(sheet+"/feature/add", "tab", "features", "name", "Lucky", "uses_max", "3", "recharge", "long", "description", "Reroll a die.")
	_, body = p.get(sheet + "?tab=features")
	wantContains(t, body, "Lucky", "3 of 3 left")
	p.post(sheet+"/text", "tab", "notes", "notes", "It began with footprints.")
	_, body = p.get(sheet + "?tab=notes")
	wantContains(t, body, "It began with footprints.")

	// Conditions and exhaustion.
	p.post(sheet+"/condition", "condition", "poisoned")
	p.post(sheet+"/exhaustion", "level", "2")
	_, body = p.get(sheet)
	wantContains(t, body, `aria-pressed="true"`, `value="2" selected`)

	// Dying and death saves.
	p.post(sheet+"/exhaustion", "level", "0")
	p.post(sheet+"/hp", "amount", "1000", "op", "damage")
	_, body = p.get(sheet)
	wantContains(t, body, "This character is dying", "Roll death save")
	res, _ = p.post(sheet+"/death", "op", "roll")
	wantRedirect(t, res, sheet)
	p.post(sheet+"/hp", "amount", "5", "op", "heal")
	_, body = p.get(sheet)
	wantContains(t, body, "5 / 26")

	// A form posted by client JS gets the sheet fragment instead of a redirect.
	p.header = map[string]string{"X-Partial": "1"}
	res, body = p.post(sheet+"/hp", "tab", "inventory", "amount", "1", "op", "heal")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "6 / 26", `tab-panel active" data-panel="inventory"`)
	if strings.Contains(body, "<html") || strings.Contains(body, "<footer") {
		t.Error("partial response contains the page layout")
	}
	res, body = p.post(sheet+"/spell/cast", "spell", "99999", "op", "cast")
	wantStatus(t, res, http.StatusBadRequest)
	wantContains(t, body, "Unknown spell")
	p.header = map[string]string{"X-Autosave": "1"}
	res, _ = p.post(sheet+"/text", "notes", "autosaved")
	wantStatus(t, res, http.StatusNoContent)
	p.header = nil
	_, body = p.get(sheet + "?tab=notes")
	wantContains(t, body, "autosaved")

	// Search fragments for client JS.
	res, body = p.get("/sheets/compendium/spells?sheet=" + strings.TrimPrefix(sheet, "/sheets/") + "&spell_q=fireball")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "Fireball", "spell/add")
	if strings.Contains(body, "<html") {
		t.Error("search should be a fragment")
	}
	res, body = p.get("/sheets/compendium/items?sheet=" + strings.TrimPrefix(sheet, "/sheets/") + "&item_q=cloak")
	wantStatus(t, res, http.StatusOK)
	wantContains(t, body, "Cloak of Protection", "item/add")

	// Delete needs confirming.
	res, _ = p.post(sheet + "/delete")
	wantStatus(t, res, http.StatusBadRequest)
	res, _ = p.post(sheet+"/delete", "confirm", "yes")
	wantRedirect(t, res, "/sheets")
	res, _ = p.get(sheet)
	wantStatus(t, res, http.StatusNotFound)
}

// Non-numeric IDs must not reach a page that would run breadcrumb.js on them.
func TestOddPathsAre404(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	for _, path := range []string{"/sheets/abc", "/sheets/%3Cimg%20src=x%20onerror=alert(1)%3E", "/admin/players/abc", "/sheets/99999"} {
		res, body := admin.get(path)
		wantStatus(t, res, http.StatusNotFound)
		if strings.Contains(body, "breadcrumb.js") {
			t.Errorf("%s renders the breadcrumb", path)
		}
	}
}

// SRD proficiency names can contain commas, and saving the edit form must not
// split them or lose what they cover.
func TestEditKeepsProficiencies(t *testing.T) {
	s := newSite(t)
	admin := s.admin()
	p, _ := s.player(admin, "alice")
	sheet := p.createCharacter("Aster")
	_, body := p.get(sheet + "/edit")
	weapons := regexp.MustCompile(`(?s)<textarea name="weapons"[^>]*>(.*?)</textarea>`).FindStringSubmatch(body)
	if weapons == nil || !strings.Contains(weapons[1], "Crossbows, light") {
		t.Fatalf("weapons field: %v", weapons)
	}
	res, body := p.post(sheet+"/edit", "name", "Aster", "alignment", "True Neutral", "size", "Medium",
		"base_str", "8", "base_dex", "14", "base_con", "13", "base_int", "15", "base_wis", "12", "base_cha", "10",
		"level_wizard", "3", "weapons", "Crossbows, light\nDaggers\r\n", "skill_arcana", "expertise", "save_int", "1")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("edit status %d\n%s", res.StatusCode, body)
	}
	_, body = p.get(sheet)
	wantContains(t, body, "Crossbows, light, Daggers")
	if strings.Contains(body, "Quarterstaffs") {
		t.Error("removed proficiency is still listed")
	}
	p.post(sheet+"/item/add", "srd", "crossbow-light")
	_, body = p.get(sheet + "?tab=inventory")
	item := regexp.MustCompile(`(?s)Crossbow, light.*?name="item" value="(\d+)"`).FindStringSubmatch(body)
	if item == nil {
		t.Fatal("no crossbow form")
	}
	p.post(sheet+"/item/equip", "item", item[1])
	_, body = p.get(sheet)
	row := regexp.MustCompile(`(?s)Crossbow, light.*?</tr>`).FindString(body)
	if row == "" || strings.Contains(row, "not proficient") {
		t.Errorf("light crossbow lost its proficiency: %s", row)
	}
}
