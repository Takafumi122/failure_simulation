import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Locale, Translations } from './types';
import { en } from './locales/en';
import { ja } from './locales/ja';

interface I18nContextValue {
  locale: Locale;
  setLocale: (loc: Locale) => void;
  toggleLocale: () => void;
  t: Translations;
}

const STORAGE_KEY = 'failurelab_locale';

function getInitialLocale(): Locale {
  try {
    // 1. Query parameter override e.g. ?lang=ja or ?lang=en
    if (typeof window !== 'undefined') {
      const urlParams = new URLSearchParams(window.location.search);
      const qLang = urlParams.get('lang')?.toLowerCase();
      if (qLang === 'en' || qLang === 'ja') {
        localStorage.setItem(STORAGE_KEY, qLang);
        return qLang;
      }
    }
    // 2. Persisted preference
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'en' || saved === 'ja') {
      return saved;
    }
  } catch {
    // ignore access error in restricted environment
  }
  // 3. Default to Japanese
  return 'ja';
}

const I18nContext = createContext<I18nContextValue>({
  locale: 'ja',
  setLocale: () => {},
  toggleLocale: () => {},
  t: ja,
});

export const I18nProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [locale, setLocaleState] = useState<Locale>(getInitialLocale);

  const setLocale = (newLoc: Locale) => {
    setLocaleState(newLoc);
    try {
      localStorage.setItem(STORAGE_KEY, newLoc);
    } catch {
      // ignore
    }
  };

  const toggleLocale = () => {
    setLocale(locale === 'en' ? 'ja' : 'en');
  };

  // Sync html lang attribute
  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.documentElement.lang = locale;
    }
  }, [locale]);

  const t = useMemo(() => (locale === 'ja' ? ja : en), [locale]);

  const value = useMemo(
    () => ({
      locale,
      setLocale,
      toggleLocale,
      t,
    }),
    [locale, t]
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
};

export const useI18n = (): I18nContextValue => {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error('useI18n must be used within an I18nProvider');
  }
  return context;
};
