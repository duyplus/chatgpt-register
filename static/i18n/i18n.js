(function () {
  const LANG_KEY = 'app_lang';

  function getLang() {
    return localStorage.getItem(LANG_KEY) || 'en';
  }

  function setLang(lang) {
    if (!['zh', 'en', 'vi'].includes(lang)) return;
    localStorage.setItem(LANG_KEY, lang);
    translateDOM();
    window.dispatchEvent(new CustomEvent('langchanged', { detail: { lang } }));
  }

  function t(key, fallback) {
    const lang = getLang();
    const dict = (window.I18N_DICTS && window.I18N_DICTS[lang]) || {};
    return dict[key] !== undefined ? dict[key] : (fallback !== undefined ? fallback : key);
  }

  function translateDOM(root) {
    const scope = root || document;

    // Text elements
    scope.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.dataset.i18n;
      const translated = t(key);
      if (translated) el.textContent = translated;
    });

    // Placeholders
    scope.querySelectorAll('[data-i18n-ph]').forEach(el => {
      const key = el.dataset.i18nPh;
      const translated = t(key);
      if (translated) el.placeholder = translated;
    });

    // Titles
    scope.querySelectorAll('[data-i18n-title]').forEach(el => {
      const key = el.dataset.i18nTitle;
      const translated = t(key);
      if (translated) el.title = translated;
    });

    // Rebuild custom enhanced selects from layout.js
    scope.querySelectorAll('select').forEach(sel => {
      if (typeof sel._rebuild === 'function') sel._rebuild();
    });

    // Sync language selector UI elements
    document.querySelectorAll('.lang-switcher-select').forEach(sel => {
      sel.value = getLang();
    });
  }

  window.i18n = { getLang, setLang, t, translateDOM };
  window.t = t;

  document.addEventListener('DOMContentLoaded', () => {
    translateDOM();
  });
})();
