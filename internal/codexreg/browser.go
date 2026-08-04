package codexreg

import (
	"chatgpt-register/internal/twofactor"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
	"strconv"
	"strings"
	"time"
)

// ErrAccountTaken Error returned when account does not exist or has been deleted/deactivated.
var ErrAccountTaken = errors.New("account does not exist or has been deleted/deactivated")

func isAccountBannedOrDeactivated(pg *rod.Page) bool {
	if pg == nil {
		return false
	}
	defer func() { _ = recover() }()
	ctx := pg.CancelTimeout().Timeout(3 * time.Second)
	const js = `() => {
		const txt = (document.body ? document.body.textContent : '').toLowerCase();
		return txt.includes('account_deactivated') ||
		       txt.includes('deleted or deactivated') ||
		       txt.includes('authentication error') ||
		       txt.includes('you do not have an account');
	}`
	res, err := ctx.Eval(js)
	if err != nil || res == nil {
		return false
	}
	return res.Value.Bool()
}

// registerBrowser Launches browser to complete ChatGPT registration and returns accessToken and 2FA secret.
func registerBrowser(ctx context.Context, in Input) (token string, tfSecret string, err error) {
	if in.IsLoginOnly {
		in.logf("🚀 Launching browser automation login & 2FA setup process...")
	} else {
		in.logf("🚀 Launching browser automation registration process...")
	}
	fp := newFingerprint(in.Email)
	l := launcher.New().
		NoSandbox(true).
		Set("disable-dev-shm-usage").
		Append("--disable-blink-features", "AutomationControlled").
		Append("--disable-infobars", "").
		Append("--no-first-run", "").
		Append("--no-default-browser-check", "").
		Set("force-webrtc-ip-handling-policy", "disable_non_proxied_udp").
		Append("--window-size", fp.windowSizeArg())
	if in.Headless {
		l = l.Set("headless", "new")
	} else {
		l = l.Headless(false)
	}
	var proxyUser, proxyPass string
	if strings.TrimSpace(in.Proxy) != "" {
		server, user, pass, perr := parseProxy(in.Proxy)
		if perr != nil {
			return "", "", fmt.Errorf("failed to parse proxy: %w", perr)
		}
		l = l.Set("proxy-server", server)
		proxyUser, proxyPass = user, pass
		in.logf("🌐 Using proxy: %s", server)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return "", "", fmt.Errorf("failed to launch Chrome: %w", err)
	}
	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		return "", "", fmt.Errorf("failed to connect to Chrome: %w", err)
	}
	defer browser.MustClose()
	var page *rod.Page
	defer func() {
		if page != nil && isAccountBannedOrDeactivated(page) {
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			err = ErrAccountTaken
		} else if r := recover(); r != nil {
			if page != nil && isAccountBannedOrDeactivated(page) {
				in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
				err = ErrAccountTaken
			} else if in.IsLoginOnly {
				err = fmt.Errorf("login & 2FA setup exception: %v", r)
			} else {
				err = fmt.Errorf("registration exception: %v", r)
			}
		}
		if err == nil || page == nil || in.SaveShot == nil {
			return
		}
		func() {
			defer func() {
				if r2 := recover(); r2 != nil {
					in.logf("⚠️ Screenshot failed (panic): %v", r2)
				}
			}()
			shotPage := page.CancelTimeout().Timeout(15 * time.Second)
			data, serr := shotPage.Screenshot(false, nil)
			if serr != nil {
				in.logf("⚠️ Screenshot failed: %v", serr)
				return
			}
			if len(data) == 0 {
				in.logf("⚠️ Screenshot failed: empty data")
				return
			}
			in.SaveShot(data)
			in.logf("📸 Saved failure screenshot")
		}()
	}()
	if proxyUser != "" || proxyPass != "" {
		go func() {
			defer func() { _ = recover() }()
			wait := browser.HandleAuth(proxyUser, proxyPass)
			_ = wait()
		}()
	}
	geo := lookupGeoIPViaRequest(in)
	acceptLang := "en-US,en;q=0.9"
	if geo != nil {
		_, acceptLang = localeForCountry(geo.CountryCode)
	}
	page = stealth.MustPage(browser)
	_, full := fp.applyUserAgent(page, browser, acceptLang)
	in.logf("🚀 Engine=%s Fingerprint: %dx%d cores=%d mem=%dG gpu=%s", full, fp.screenW, fp.screenH, fp.cores, fp.memory, fp.gpu.renderer)
	fp.inject(page)
	if geo != nil {
		applyGeo(page, geo, in)
	}
	page = page.Timeout(120 * time.Second)
	if in.IsLoginOnly {
		in.logf("🌐 Opening ChatGPT login page...")
	} else {
		in.logf("🌐 Opening ChatGPT registration page...")
	}
	page.MustNavigate("https://chatgpt.com/auth/login")
	page.MustWaitLoad()
	page.MustElement("#email").MustWaitVisible()
	if in.IsLoginOnly {
		in.logf("✅ Login page loaded")
	} else {
		in.logf("✅ Registration page loaded")
	}
	page.MustElement("#email").MustInput(in.Email)
	page.MustElement("button[type='submit']").MustEval(`() => this.click()`)
	in.logf("📩 Submitted email, waiting for next step...")
	time.Sleep(2 * time.Second)
	if isAccountBannedOrDeactivated(page) {
		in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
		return "", "", ErrAccountTaken
	}
	codeReady := false
	passwordDone := false
	skipCodeInput := false
	for attempt := 0; attempt < 5 && !codeReady; attempt++ {
		if isAccountBannedOrDeactivated(page) {
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			return "", "", ErrAccountTaken
		}
		pg := page.CancelTimeout().Timeout(45 * time.Second)
		state := ""
		pg.Race().Element("input[name='code']").MustHandle(func(_ *rod.Element) {
			state = "code"
		}).Element("input[type='password']").MustHandle(func(_ *rod.Element) {
			state = "password"
		}).ElementR("body", "account_deactivated|deleted or deactivated|Authentication Error|You do not have an account").MustHandle(func(_ *rod.Element) {
			state = "disabled"
		}).Element("textarea[name='prompt-textarea']").MustHandle(func(_ *rod.Element) {
			state = "ready"
		}).Element("input[name='name']").MustHandle(func(_ *rod.Element) {
			state = "profile"
		}).MustDo()
		switch state {
		case "code":
			codeReady = true
		case "password":
			if passwordDone {
				time.Sleep(2 * time.Second)
				if isAccountBannedOrDeactivated(page) {
					in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
					return "", "", ErrAccountTaken
				}
				continue
			}
			in.logf("🔑 Password page appeared, submitting password automatically")
			pw := pg.MustElement("input[type='password']")
			pw.MustSelectAllText().MustInput(in.Password)
			pg.MustElement("button[type='submit']").MustEval(`() => this.click()`)
			passwordDone = true
			time.Sleep(2 * time.Second)
			if isAccountBannedOrDeactivated(page) {
				in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
				return "", "", ErrAccountTaken
			}
		case "disabled":
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			return "", "", ErrAccountTaken
		case "ready", "profile":
			codeReady = true
			skipCodeInput = true
		}
	}
	if isAccountBannedOrDeactivated(page) {
		in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
		return "", "", ErrAccountTaken
	}
	if !codeReady {
		return "", "", fmt.Errorf("timeout waiting for verification code or password input box")
	}
	if !skipCodeInput {
		in.logf("📨 Verification code input box appeared, reading code from email...")
		code, err := in.FetchCode(ctx)
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch verification code: %w", err)
		}
		code = strings.TrimSpace(code)
		if code == "" {
			return "", "", fmt.Errorf("verification code is empty")
		}
		page = page.CancelTimeout().Timeout(120 * time.Second)
		page.MustElement("input[name='code']").MustInput(code)
		page.MustElement("button[type='submit']").MustEval(`() => this.click()`)
		in.logf("🔑 Submitted verification code")
		time.Sleep(2 * time.Second)
		if isAccountBannedOrDeactivated(page) {
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			return "", "", ErrAccountTaken
		}
	}

	ready := false
	for attempt := 0; attempt < 8 && !ready; attempt++ {
		if isAccountBannedOrDeactivated(page) {
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			return "", "", ErrAccountTaken
		}
		pg := page.CancelTimeout().Timeout(60 * time.Second)
		state := ""
		pg.Race().Element("textarea[name='prompt-textarea']").MustHandle(func(_ *rod.Element) {
			state = "ready"
		}).ElementR("body", "account_deactivated|deleted or deactivated|Authentication Error|You do not have an account").MustHandle(func(_ *rod.Element) {
			state = "disabled"
		}).ElementR("button", "Try again|Thử lại|重试").MustHandle(func(_ *rod.Element) {
			state = "retry"
		}).Element("input[name='name']").MustHandle(func(_ *rod.Element) {
			state = "profile"
		}).ElementR("button", "Finish creating account|Hoàn tất tạo tài khoản|完成创建账户|完成创建账号").MustHandle(func(_ *rod.Element) {
			state = "profile"
		}).MustDo()
		switch state {
		case "ready":
			ready = true
		case "disabled":
			in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
			return "", "", ErrAccountTaken
		case "retry":
			in.logf("🔄 Page error (Operation timed out), clicking Try again to continue")
			pg.MustElementR("button", "Try again|Thử lại|重试").MustEval(`() => this.click()`)
			time.Sleep(3 * time.Second)
		case "profile":
			in.logf("👤 Profile completion page appeared")
			fillProfile(pg, in)
			time.Sleep(2 * time.Second)
		}
	}
	if isAccountBannedOrDeactivated(page) {
		in.logf("🛑 Account deactivated/banned by OpenAI (Authentication Error: account_deactivated)")
		return "", "", ErrAccountTaken
	}
	if !ready {
		return "", "", fmt.Errorf("timeout waiting for ChatGPT main interface")
	}
	in.logf("✅ ChatGPT main interface ready")
	finalPw := setupChatGPTPassword(ctx, page, in)
	if finalPw != "" {
		in.Password = finalPw
	}
	tfSecret = setupChatGPT2FA(ctx, page, in)
	if tfSecret == "" {
		return "", "", fmt.Errorf("failed to enable 2FA (MFA) on ChatGPT account")
	}
	if tfSecret == "already_enabled" {
		tfSecret = in.TwoFactorSecret
	}
	in.logf("🔑 Extracting account authentication session data...")
	page = page.CancelTimeout().Timeout(60 * time.Second)
	page.MustNavigate("https://chatgpt.com/api/auth/session")
	page.MustWaitLoad()
	body := page.MustElement("body").MustText()
	var sessionData map[string]any
	if err := json.Unmarshal([]byte(body), &sessionData); err != nil {
		return "", tfSecret, fmt.Errorf("failed to parse session JSON: %w", err)
	}
	accessToken, ok := sessionData["accessToken"].(string)
	if !ok || accessToken == "" {
		return "", tfSecret, fmt.Errorf("account authentication session data not found, login might have failed")
	}
	in.logf("🔑 Account authentication session data retrieved successfully")
	return accessToken, tfSecret, nil
}

// fillProfile Fills out profile / birthdate completion page.
func fillProfile(pg *rod.Page, in Input) {
	age := 18 + ri(28)
	if v, err := strconv.Atoi(strings.TrimSpace(in.Age)); err == nil && v >= 18 {
		age = v
	}
	bday := time.Now().AddDate(-age, 0, 0).AddDate(0, 0, -ri(365))
	iso := bday.Format("2006-01-02")
	mmddyyyy := bday.Format("01022006")
	const js = `(fullName) => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const setVal = (el, v) => {
			const d = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
			d.set.call(el, v);
			el.dispatchEvent(new Event('input', {bubbles:true}));
			el.dispatchEvent(new Event('change', {bubbles:true}));
		};
		const byLabel = (sub) => {
			sub = norm(sub);
			for (const l of document.querySelectorAll('label')) {
			if (norm(l.textContent).includes(sub)) {
				let i = l.querySelector('input');
				if (!i && l.htmlFor) i = document.getElementById(l.htmlFor);
				if (!i) { const c = l.closest('div,fieldset,section'); if (c) i = c.querySelector('input'); }
				if (i) return i;
			}
			}
			for (const i of document.querySelectorAll('input')) {
			const a = norm(i.getAttribute('aria-label')) + ' ' + norm(i.placeholder) + ' ' + norm(i.name);
			if (a.includes(sub)) return i;
			}
			return null;
		};
		document.querySelectorAll('[data-devin-target]').forEach(e => e.removeAttribute('data-devin-target'));
		const nameInput = byLabel('full name') || document.querySelector('input[name="name"]') || byLabel('name');
		if (nameInput) setVal(nameInput, fullName);
		const ageInput = document.querySelector('input[name="age"]');
		const bInput = document.querySelector('input[name="birthday"],input[name="birthdate"],input[name="dob"],input[type="date"]') ||
						byLabel('birthday') || byLabel('date of birth') || byLabel('birth');
		const res = {name: !!nameInput, kind: '', isDate: false};
		if (ageInput && ageInput !== bInput) {
			ageInput.setAttribute('data-devin-target', '1');
			res.kind = 'age';
		} else if (bInput) {
			bInput.setAttribute('data-devin-target', '1');
			res.kind = 'birthday';
			res.isDate = (bInput.type || '').toLowerCase() === 'date';
		}
		return JSON.stringify(res);
	}`
	raw := pg.MustEval(js, in.FullName).Str()
	var info struct {
		Name   bool   `json:"name"`
		Kind   string `json:"kind"`
		IsDate bool   `json:"isDate"`
	}
	_ = json.Unmarshal([]byte(raw), &info)
	in.logf("Profile fields: name=%v type=%q isDate=%v", info.Name, info.Kind, info.IsDate)
	switch info.Kind {
	case "age":
		pg.MustElement("[data-devin-target]").MustSelectAllText().MustInput(in.Age)
		in.logf("👤 Profile filled (name + age=%s)", in.Age)
	case "birthday":
		if info.IsDate {
			pg.MustEval(`(v) => {
				const el = document.querySelector('[data-devin-target]');
				const d = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
				d.set.call(el, v);
				el.dispatchEvent(new Event('input', {bubbles:true}));
				el.dispatchEvent(new Event('change', {bubbles:true}));
			}`, iso)
		} else {
			el := pg.MustElement("[data-devin-target]")
			el.MustSelectAllText().MustInput(mmddyyyy)
		}
		in.logf("Profile filled (name + birthday=%s)", iso)
	default:
		in.logf("Profile page age/birthday field not identified, attempting submit directly")
	}
	pg.MustElement("button[type='submit']").MustEval(`() => this.click()`)
}

// setupChatGPTPassword Navigates to ChatGPT Security settings, clicks Set Password button if available,
// handles email code verification if required, sets a new password on reset-password/new-password page, and returns final password.
func setupChatGPTPassword(ctx context.Context, page *rod.Page, in Input) string {
	in.logf("🔐 Checking password setup status in ChatGPT Security settings...")
	defer func() {
		if r := recover(); r != nil {
			in.logf("⚠️ Password setup encountered exception: %v", r)
		}
	}()
	pg := page.CancelTimeout().Timeout(45 * time.Second)
	pg.MustNavigate("https://chatgpt.com/#settings/Security")
	time.Sleep(5 * time.Second)
	handleCloudflare(page, in)

	// Step 1: Check password status in Security settings (Thêm > vs ****** >)
	const findPasswordBtnJS = `() => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const triggerClick = el => {
			try { el.click(); } catch(e){}
			try {
				const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
				el.dispatchEvent(ev);
			} catch(e){}
		};
		const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
		const dialog = dialogs.length > 0 ? dialogs[dialogs.length - 1] : document.body;

		// Scroll to top of dialog
		try {
			dialog.scrollTop = 0;
			for (const child of dialog.querySelectorAll('div, section')) {
				child.scrollTop = 0;
			}
		} catch(e){}

		// Priority 1: Check exact data-testid match first!
		const pwBtnTestId = dialog.querySelector('[data-testid="password-setting"]');
		if (pwBtnTestId) {
			const txt = norm(pwBtnTestId.textContent);
			if (txt.includes('*') || txt.includes('●') || txt.includes('đổi') || txt.includes('change') || txt.includes('edit')) {
				return "already_has_password";
			}
			triggerClick(pwBtnTestId);
			return "clicked_testid";
		}

		// Priority 2: Search for row container containing "mật khẩu" / "password"
		for (const container of dialog.querySelectorAll('div, section, li, tr')) {
			const cText = norm(container.textContent);
			if ((cText.includes('mật khẩu') || cText.includes('password')) &&
			    !cText.includes('khóa bảo mật') && !cText.includes('passkey') &&
			    !cText.includes('xác thực') && !cText.includes('mfa') && !cText.includes('phiên đăng nhập') && !cText.includes('device code')) {
				if (cText.includes('*') || cText.includes('●') || cText.includes('đổi') || cText.includes('change') || cText.includes('edit')) {
					return "already_has_password";
				}
				for (const btn of container.querySelectorAll('button, a, div[role="button"], span')) {
					const bText = norm(btn.textContent);
					if (bText.includes('*') || bText.includes('●') || bText.includes('đổi') || bText.includes('change')) {
						return "already_has_password";
					}
					if (bText.includes('thêm') || bText.includes('add') || bText.includes('set') || bText.includes('cài đặt') || bText.includes('tạo')) {
						triggerClick(btn);
						return "clicked_add_password";
					}
				}
				const btn = container.querySelector('button, a, div[role="button"]');
				if (btn) {
					triggerClick(btn);
					return "clicked_row_btn";
				}
			}
		}
		// Priority 3: Direct button search fallback
		for (const el of dialog.querySelectorAll('button, a, div[role="button"], span')) {
			const txt = norm(el.textContent);
			if (txt.includes('*') || txt.includes('●') || txt.includes('change password') || txt.includes('đổi mật khẩu')) {
				return "already_has_password";
			}
			if (txt.includes('set password') || txt.includes('cài đặt mật khẩu') || txt.includes('thêm mật khẩu') || txt.includes('tạo mật khẩu')) {
				triggerClick(el);
				return "clicked_direct";
			}
		}
		return "not_found";
	}`

	pwStatus := "not_found"
	for attempt := 0; attempt < 8; attempt++ {
		pwStatus = pg.MustEval(findPasswordBtnJS).Str()
		if pwStatus != "not_found" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	in.logf("🔑 Password setting button status: %s", pwStatus)
	if pwStatus == "already_has_password" {
		in.logf("ℹ️ Account already has password set (****** >), skipping password setup and proceeding to 2FA...")
		return in.Password
	}
	time.Sleep(5 * time.Second)

	// Step 2: Check if redirected to email verification or password reset page
	for attempt := 0; attempt < 5; attempt++ {
		url := pg.MustInfo().URL
		if strings.Contains(url, "email-verification") || strings.Contains(url, "reset-password") || pg.MustHas("input[name='code']") || pg.MustHas("input[type='password']") {
			break
		}
		time.Sleep(2 * time.Second)
	}

	url := pg.MustInfo().URL
	in.logf("🌐 Current URL after clicking password setting: %s", url)

	// Step 3: Handle email verification code if input[name='code'] appears or URL is email-verification
	if strings.Contains(url, "email-verification") || pg.MustHas("input[name='code']") || pg.MustHas("input") {
		in.logf("📨 Email verification page appeared for password setup, reading code from email...")
		code, err := in.FetchCode(ctx)
		if err != nil {
			in.logf("⚠️ Failed to fetch verification code for password setup: %v", err)
			return in.Password
		}
		code = strings.TrimSpace(code)
		if code != "" {
			pg = pg.CancelTimeout().Timeout(60 * time.Second)
			if el, err := pg.Element("input[name='code']"); err == nil && el != nil {
				el.MustInput(code)
			} else if el, err := pg.Element("input"); err == nil && el != nil {
				el.MustInput(code)
			}
			if btn, err := pg.Element("button[type='submit']"); err == nil && btn != nil {
				btn.MustEval(`() => this.click()`)
			} else if btn, err := pg.Element("button"); err == nil && btn != nil {
				btn.MustEval(`() => this.click()`)
			}
			in.logf("🔑 Submitted email verification code for password setup")
			time.Sleep(5 * time.Second)
			handleCloudflare(page, in)
		}
	}

	// Step 4: Handle auth.openai.com/reset-password/new-password page (Thêm mật khẩu)
	newPass := GenPassword(16)
	for attempt := 0; attempt < 5; attempt++ {
		if strings.Contains(pg.MustInfo().URL, "reset-password") || pg.MustHas("input[type='password']") {
			break
		}
		time.Sleep(2 * time.Second)
	}

	currURL := pg.MustInfo().URL
	in.logf("🌐 Setting password on page: %s", currURL)

	const fillNewPassJS = `(pass) => {
		const setVal = (el, v) => {
			try { el.focus(); } catch(e){}
			const d = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
			if (d && d.set) d.set.call(el, v); else el.value = v;
			el.dispatchEvent(new Event('input', {bubbles:true, composed:true}));
			el.dispatchEvent(new Event('change', {bubbles:true, composed:true}));
		};
		const inputs = Array.from(document.querySelectorAll('input[type="password"], input[name*="password"]'));
		if (inputs.length >= 2) {
			setVal(inputs[0], pass);
			setVal(inputs[1], pass);
			return "filled_both";
		} else if (inputs.length === 1) {
			setVal(inputs[0], pass);
			return "filled_one";
		}
		return "no_inputs";
	}`

	fillRes := pg.MustEval(fillNewPassJS, newPass).Str()
	in.logf("🔑 Password input fields result: %s", fillRes)
	if fillRes != "no_inputs" {
		const submitPassJS = `() => {
			const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
			for (const btn of document.querySelectorAll('button')) {
				const txt = norm(btn.textContent);
				if (txt.includes('tiếp tục') || txt.includes('continue') || txt.includes('xác nhận') || txt.includes('submit') || txt.includes('hoàn tất') || btn.type === 'submit') {
					try { btn.removeAttribute('disabled'); } catch(e){}
					try { btn.click(); } catch(e){}
					try {
						const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
						btn.dispatchEvent(ev);
					} catch(e){}
					return true;
				}
			}
			return false;
		}`
		submittedPw := pg.MustEval(submitPassJS).Bool()
		in.logf("🔑 Submitted new password form: %v", submittedPw)
		time.Sleep(5 * time.Second)
		in.logf("✅ Password set successfully for account!")
		return newPass
	}
	return in.Password
}

// handleCloudflare Checks for Cloudflare Turnstile challenge box and automatically clicks to pass.
func handleCloudflare(page *rod.Page, in Input) {
	defer func() { _ = recover() }()
	pg := page.CancelTimeout().Timeout(10 * time.Second)
	const checkCFJS = `() => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const bodyTxt = norm(document.body ? document.body.textContent : '');
		if (bodyTxt.includes('verifying...') || bodyTxt.includes('cloudflare') || bodyTxt.includes('just a moment')) {
			return true;
		}
		for (const iframe of document.querySelectorAll('iframe')) {
			const src = (iframe.src || '').toLowerCase();
			const title = (iframe.title || '').toLowerCase();
			if (src.includes('cloudflare') || src.includes('turnstile') || title.includes('cloudflare')) {
				return true;
			}
		}
		return false;
	}`
	isCF := pg.MustEval(checkCFJS).Bool()
	if !isCF {
		return
	}
	in.logf("🛡️ Cloudflare Turnstile verification challenge detected, solving automatically...")
	for attempt := 0; attempt < 5; attempt++ {
		// Attempt 1: Frame element selection & click
		if el, err := pg.Element("iframe[src*='cloudflare'], iframe[src*='turnstile'], iframe[title*='Cloudflare']"); err == nil && el != nil {
			if frame, err := el.Frame(); err == nil && frame != nil {
				if cb, err := frame.Element("input[type='checkbox'], .mark, #challenge-stage"); err == nil && cb != nil {
					cb.MustClick()
					in.logf("🛡️ Clicked Cloudflare Turnstile checkbox inside frame")
					time.Sleep(3 * time.Second)
					break
				}
			}
		}
		// Attempt 2: Dispatch MouseEvent to center of Cloudflare iframe via JS
		const clickCFJS = `() => {
			for (const iframe of document.querySelectorAll('iframe')) {
				const src = (iframe.src || '').toLowerCase();
				const title = (iframe.title || '').toLowerCase();
				if (src.includes('cloudflare') || src.includes('turnstile') || title.includes('cloudflare')) {
					try {
						const rect = iframe.getBoundingClientRect();
						if (rect.width > 0 && rect.height > 0) {
							const clickEvent = new MouseEvent('click', {
								clientX: rect.left + rect.width / 2,
								clientY: rect.top + rect.height / 2,
								bubbles: true
							});
							iframe.dispatchEvent(clickEvent);
							return true;
						}
					} catch(e){}
				}
			}
			return false;
		}`
		if pg.MustEval(clickCFJS).Bool() {
			in.logf("🛡️ Dispatched click to Cloudflare Turnstile widget")
			time.Sleep(3 * time.Second)
			break
		}
		time.Sleep(2 * time.Second)
	}

	time.Sleep(3 * time.Second)
}

// setupChatGPT2FA Navigates to ChatGPT Security settings, enables MFA, extracts 2FA secret,
// fetches TOTP code from 2fa.live, submits it, and returns the 2FA secret key.
func setupChatGPT2FA(ctx context.Context, page *rod.Page, in Input) string {
	in.logf("🔒 Enabling 2FA (MFA) on ChatGPT account...")
	defer func() {
		if r := recover(); r != nil {
			in.logf("⚠️ 2FA setup encountered exception: %v", r)
		}
	}()

	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			in.logf("🔄 Retrying 2FA setup from #settings (Attempt %d/3)...", attempt)
		}
		secretKey, isAlreadyOn := trySetupChatGPT2FAOnce(ctx, page, in, attempt)
		if isAlreadyOn {
			if in.TwoFactorSecret != "" {
				in.logf("ℹ️ 2FA is already enabled on this account and secret key exists in DB")
				return in.TwoFactorSecret
			}
			in.logf("⚠️ 2FA is enabled on ChatGPT but secret key is missing in DB. Resetting 2FA to extract new secret key...")
			if resetChatGPT2FA(ctx, page, in) {
				time.Sleep(3 * time.Second)
				continue
			}
			return "already_enabled"
		}
		if secretKey != "" {
			in.logf("✅ 2FA setup completed successfully on ChatGPT account!")
			return secretKey
		}
		if attempt < 3 {
			in.logf("⚠️ 2FA setup attempt %d/3 failed, resetting settings modal and retrying...", attempt)
			// Reset open modals by clicking close/cancel
			_ = page.MustEval(`() => {
				for (const btn of document.querySelectorAll('button')) {
					const ph = (btn.getAttribute('aria-label')||'').toLowerCase();
					const txt = (btn.textContent||'').toLowerCase();
					if (ph.includes('close') || txt === 'cancel' || txt === 'hủy' || txt === 'close') {
						try { btn.click(); } catch(e){}
					}
				}
			}`)
			time.Sleep(5 * time.Second)
		}
	}

	in.logf("❌ Failed to enable 2FA (MFA) on ChatGPT account after 3 attempts")
	return ""
}

func resetChatGPT2FA(ctx context.Context, page *rod.Page, in Input) bool {
	defer func() { _ = recover() }()
	pg := page.CancelTimeout().Timeout(20 * time.Second)
	const turnOffJS = `() => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const triggerClick = el => {
			try { el.click(); } catch(e){}
			try {
				const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
				el.dispatchEvent(ev);
			} catch(e){}
		};
		const dialog = document.querySelector('[role="dialog"]') || document.body;
		const sw = dialog.querySelector('[data-testid="mfa-authenticator-toggle"], button[role="switch"], input[type="checkbox"]');
		if (sw) {
			const checked = sw.getAttribute('aria-checked') === 'true' || sw.getAttribute('data-state') === 'checked' || sw.checked;
			if (checked) {
				triggerClick(sw);
				return "clicked_off";
			}
		}
		return "not_checked";
	}`
	res := pg.MustEval(turnOffJS).Str()
	if res == "clicked_off" {
		in.logf("🔄 Clicked 2FA switch to disable existing 2FA...")
		time.Sleep(2 * time.Second)
		const confirmDisableJS = `() => {
			const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
			for (const btn of document.querySelectorAll('button')) {
				const txt = norm(btn.textContent);
				if (txt.includes('disable') || txt.includes('tắt') || txt.includes('ngừng') || txt.includes('turn off') || txt.includes('confirm') || txt.includes('đồng ý')) {
					try { btn.click(); } catch(e){}
					return true;
				}
			}
			return false;
		}`
		_ = pg.MustEval(confirmDisableJS).Bool()
		time.Sleep(3 * time.Second)
		return true
	}
	return false
}

func trySetupChatGPT2FAOnce(ctx context.Context, page *rod.Page, in Input, attemptNum int) (string, bool) {
	pg := page.CancelTimeout().Timeout(45 * time.Second)
	in.logf("🌐 Opening ChatGPT settings (#settings)...")
	pg.MustNavigate("https://chatgpt.com/#settings")
	time.Sleep(5 * time.Second)
	handleCloudflare(page, in)
	in.logf("🌐 Navigating to Security tab (#settings/Security)...")
	pg.MustNavigate("https://chatgpt.com/#settings/Security")
	time.Sleep(5 * time.Second)
	handleCloudflare(page, in)

	// Step 1: Find MFA switch button ("data-testid=mfa-authenticator-toggle" / "Authenticator app" / "Ứng dụng xác minh")
	const findSwitchJS = `() => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const triggerClick = el => {
			try { el.click(); } catch(e){}
			try {
				const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
				el.dispatchEvent(ev);
			} catch(e){}
		};
		const dialog = document.querySelector('[role="dialog"]') || document.body;
		// Scroll dialog to middle/top to ensure switch button is rendered
		try {
			dialog.scrollTop = 100;
			for (const child of dialog.querySelectorAll('div, section')) {
				child.scrollTop = 100;
			}
		} catch(e){}
		// Priority 1: Exact data-testid match
		const toggleByTestId = dialog.querySelector('[data-testid="mfa-authenticator-toggle"]');
		if (toggleByTestId) {
			const checked = toggleByTestId.getAttribute('aria-checked') === 'true' || toggleByTestId.getAttribute('data-state') === 'checked' || toggleByTestId.checked;
			if (!checked) {
				triggerClick(toggleByTestId);
				return "clicked_testid";
			}
			return "already_on";
		}
		// Priority 2: Standard switch / checkbox elements near MFA / Authenticator keywords
		for (const container of dialog.querySelectorAll('div, section, li, label, tr')) {
			const parentText = norm(container.textContent);
			if (parentText.includes('authenticator') || parentText.includes('mfa') || parentText.includes('xác minh') || parentText.includes('đa yếu tố') || parentText.includes('multi-factor') || parentText.includes('factor')) {
				const sw = container.querySelector('button[role="switch"], input[type="checkbox"], [data-testid*="mfa"], [data-testid*="toggle"]');
				if (sw) {
					const checked = sw.getAttribute('aria-checked') === 'true' || sw.getAttribute('data-state') === 'checked' || sw.checked;
					if (!checked) {
						triggerClick(sw);
						return "clicked";
					}
					return "already_on";
				}
			}
		}
		// Priority 3: Any switch element in dialog
		for (const sw of dialog.querySelectorAll('button[role="switch"], input[type="checkbox"]')) {
			const checked = sw.getAttribute('aria-checked') === 'true' || sw.getAttribute('data-state') === 'checked' || sw.checked;
			if (!checked) {
				triggerClick(sw);
				return "clicked_fallback";
			}
			return "already_on";
		}
		return "not_found";
	}`

	status := "not_found"
	for attempt := 0; attempt < 12; attempt++ {
		status = pg.MustEval(findSwitchJS).Str()
		if status != "not_found" {
			break
		}
		// If settings dialog is open, try clicking Security tab directly
		if attempt == 3 || attempt == 7 {
			const clickSecurityTabJS = `() => {
				const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
				const dialog = document.querySelector('[role="dialog"]');
				if (dialog) {
					for (const item of dialog.querySelectorAll('button, a, div[role="button"], span, [role="tab"]')) {
						const t = norm(item.textContent);
						if (t.includes('bảo mật') || t.includes('security') || t.includes('mfa')) {
							try { item.click(); } catch(e){}
							return "clicked_security_tab";
						}
					}
				}
				return "none";
			}`
			_ = pg.MustEval(clickSecurityTabJS).Str()
		}
		time.Sleep(1 * time.Second)
	}

	in.logf("🔒 MFA switch status: %s", status)
	if status == "already_on" {
		return "", true
	}
	if status == "not_found" {
		in.logf("⚠️ Could not locate MFA switch button in ChatGPT settings (attempt %d)", attemptNum)
		return "", false
	}
	time.Sleep(5 * time.Second)
	handleCloudflare(page, in)

	// Step 1b: Check if toggling MFA switch redirected to email verification or input[name='code']
	isEmailVerification := false
	for attempt := 0; attempt < 5; attempt++ {
		currUrl := pg.MustInfo().URL
		if strings.Contains(currUrl, "email-verification") || pg.MustHas("input[name='code']") {
			isEmailVerification = true
			break
		}
		if pg.MustHas("[role='dialog']") {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if isEmailVerification {
		in.logf("📨 MFA setup requested email verification, reading code from email...")
		code, err := in.FetchCode(ctx)
		if err != nil {
			in.logf("⚠️ Failed to fetch verification code for MFA setup: %v", err)
			return "", false
		}
		code = strings.TrimSpace(code)
		if code != "" {
			pg = pg.CancelTimeout().Timeout(60 * time.Second)
			if el, err := pg.Element("input[name='code']"); err == nil && el != nil {
				el.MustInput(code)
			}
			if btn, err := pg.Element("button[type='submit']"); err == nil && btn != nil {
				btn.MustEval(`() => this.click()`)
			}
			in.logf("🔑 Submitted email verification code for MFA setup")
			time.Sleep(5 * time.Second)
			handleCloudflare(page, in)
			// Ensure returned to Security page
			if !strings.Contains(pg.MustInfo().URL, "#settings/Security") {
				pg.MustNavigate("https://chatgpt.com/#settings/Security")
				time.Sleep(5 * time.Second)
				handleCloudflare(page, in)
			}
		}
	}

	// Step 2: Extract 2FA Secret Key from modal (Priority 1: QR Code link, Priority 2: Trouble scanning? button)
	const extractSecretJS = `() => {
		const clean = s => (s||'').replace(/^(copy\s*code|sao\s*chép\s*mã|secret\s*key|code)\s*/i, '').replace(/[\s-]+/g, '').trim().toUpperCase();
		const isBase32 = s => {
			if (!s || s.length < 16 || s.length > 64) return false;
			if (/SECURITY|LOGIN|AUTHENTICATOR|SETTINGS|PRIVACY|TROUBLE|PROBLEM|PASSWORD|PARENTAL|DEVELOPER|ELEVATED|RISK|CONNECT|ENTER|COPY|MFA|CODE/.test(s)) return false;
			return /^[A-Z2-7]{16,64}$/.test(s);
		};
		// Priority 1: Check QR code link href containing secret parameter (otpauth://totp/...?secret=XXXXX...)
		for (const a of document.querySelectorAll('a[href*="otpauth://"], a[href*="secret="]')) {
			const href = a.href || a.getAttribute('href') || '';
			const match = /secret=([A-Z2-7]{16,64})/i.exec(href);
			if (match && match[1]) {
				const s = match[1].toUpperCase();
				if (isBase32(s)) return s;
			}
		}
		const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
		const dialog = dialogs.length > 0 ? dialogs[dialogs.length - 1] : document.body;

		// Priority 2: Check leaf elements and buttons inside modal
		for (const el of dialog.querySelectorAll('*')) {
			if (el.children.length === 0 || el.tagName === 'INPUT' || el.tagName === 'CODE' || el.tagName === 'BUTTON') {
				const txt = clean(el.value || el.textContent);
				if (isBase32(txt)) return txt;
			}
		}
		// Priority 3: Check elements near Copy/Sao chép
		for (const container of dialog.querySelectorAll('div, section')) {
			const normText = (container.textContent || '').toLowerCase();
			if (normText.includes('sao chép') || normText.includes('copy') || normText.includes('mã bí mật') || normText.includes('secret key')) {
				for (const el of container.querySelectorAll('button, div, span, code, input, p')) {
					const txt = clean(el.value || el.textContent);
					if (isBase32(txt)) return txt;
				}
			}
		}
		return "";
	}`

	const clickTroubleJS = `() => {
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const triggerClick = el => {
			try { el.click(); } catch(e){}
			try {
				const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
				el.dispatchEvent(ev);
			} catch(e){}
		};
		const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
		for (let i = dialogs.length - 1; i >= 0; i--) {
			const d = dialogs[i];
			for (const el of d.querySelectorAll('a, button, span, div, p')) {
				const txt = norm(el.textContent);
				if (txt.includes('gặp vấn đề') || txt.includes('trouble scanning') || txt.includes('problem scanning') || txt.includes('having trouble')) {
					triggerClick(el);
					return true;
				}
			}
		}
		return false;
	}`

	secretKey := ""
	for attempt := 0; attempt < 10; attempt++ {
		// Try extracting secret (from QR code link or existing modal content)
		secretKey = pg.MustEval(extractSecretJS).Str()
		secretKey = twofactor.CleanSecret(secretKey)
		if secretKey != "" {
			break
		}
		// If not found yet, click "Trouble scanning?" to expose manual key
		if attempt == 2 || attempt == 5 {
			_ = pg.MustEval(clickTroubleJS).Bool()
		}
		time.Sleep(1 * time.Second)
	}

	if secretKey == "" {
		in.logf("⚠️ 2FA Secret Key could not be read from ChatGPT modal (attempt %d)", attemptNum)
		return "", false
	}
	in.logf("🔑 Extracted ChatGPT 2FA Secret Key: %s", secretKey)

	// Step 4: Get TOTP code via 2fa.live
	totpCode, err := twofactor.GetCode(ctx, secretKey)
	if err != nil {
		in.logf("⚠️ Failed to generate 2FA code via 2fa.live: %v (attempt %d)", err, attemptNum)
		return "", false
	}
	in.logf("🔢 Generated 2FA code via 2fa.live: %s", totpCode)

	// Step 5: Fill TOTP code into modal step 2 input box and click Verify / Xác minh
	const submitCodeJS = `(code) => {
		const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
		const dialog = dialogs.length > 0 ? dialogs[dialogs.length - 1] : document.body;
		const setVal = (el, v) => {
			try { el.focus(); } catch(e){}
			const d = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value');
			if (d && d.set) d.set.call(el, v); else el.value = v;
			el.dispatchEvent(new Event('input', {bubbles:true, composed:true}));
			el.dispatchEvent(new Event('change', {bubbles:true, composed:true}));
			el.dispatchEvent(new KeyboardEvent('keydown', {key:'Enter', keyCode:13, bubbles:true}));
			el.dispatchEvent(new KeyboardEvent('keyup', {key:'Enter', keyCode:13, bubbles:true}));
		};
		const inputs = dialog.querySelectorAll('input');
		let codeInp = null;
		for (const i of inputs) {
			const ph = (i.placeholder || '').toLowerCase();
			if (i.type === 'text' || i.type === 'number' || ph.includes('6') || ph.includes('nhập mã') || ph.includes('code')) {
				codeInp = i;
				break;
			}
		}
		if (!codeInp && inputs.length > 0) codeInp = inputs[inputs.length - 1];
		if (codeInp) {
			setVal(codeInp, code);
		} else {
			return false;
		}
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		for (const btn of dialog.querySelectorAll('button')) {
			const txt = norm(btn.textContent);
			if (txt === 'xác minh' || txt === 'verify' || txt.includes('xác minh') || txt.includes('verify') || txt.includes('continue') || txt.includes('tiếp tục')) {
				try { btn.removeAttribute('disabled'); } catch(e){}
				try { btn.click(); } catch(e){}
				try {
					const ev = new MouseEvent('click', { bubbles: true, cancelable: true, view: window });
					btn.dispatchEvent(ev);
				} catch(e){}
				return true;
			}
		}
		return false;
	}`

	submitted := false
	for attempt := 0; attempt < 5; attempt++ {
		if pg.MustEval(submitCodeJS, totpCode).Bool() {
			submitted = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	in.logf("🔑 Submitted 2FA TOTP code to ChatGPT: %v", submitted)
	if !submitted {
		in.logf("⚠️ Failed to submit 2FA TOTP code to ChatGPT (attempt %d)", attemptNum)
		return "", false
	}
	time.Sleep(5 * time.Second)

	// Step 6: Verify 2FA status from DOM
	const checkSuccessJS = `() => {
		const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
		if (dialogs.length === 0) return true;
		const dialog = dialogs[dialogs.length - 1];
		const norm = s => (s||'').replace(/\s+/g,' ').trim().toLowerCase();
		const txt = norm(dialog.textContent);
		if (txt.includes('invalid') || txt.includes('incorrect') || txt.includes('error') || txt.includes('không hợp lệ') || txt.includes('lỗi')) {
			return false;
		}
		return true;
	}`
	isSuccess := pg.MustEval(checkSuccessJS).Bool()
	if !isSuccess {
		in.logf("⚠️ 2FA TOTP code was rejected by ChatGPT (attempt %d)", attemptNum)
		return "", false
	}
	return secretKey, false
}
