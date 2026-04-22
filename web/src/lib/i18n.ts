import {
  createContext,
  createElement,
  startTransition,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type PropsWithChildren,
} from "react";

import {
  en,
  type TranslationDictionary,
  type TranslationKey,
} from "./i18n-dictionaries/en";

export type Locale = "en" | "ko";

export const defaultLocale: Locale = "en";
export const localeStorageKey = "mission.frontend.locale";

type TranslationParams = Record<string, number | string>;

const dictionaryCache: Partial<Record<Locale, TranslationDictionary>> = {
  en,
};

type I18nContextValue = {
  formatDateTime: (
    value: Date | number,
    options?: Intl.DateTimeFormatOptions,
  ) => string;
  formatNumber: (
    value: bigint | number,
    options?: Intl.NumberFormatOptions,
  ) => string;
  locale: Locale;
  localeTag: string;
  setLocale: (next: Locale) => void;
  t: (key: TranslationKey, params?: TranslationParams) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function interpolate(
  template: string,
  params?: TranslationParams,
): string {
  if (!params) {
    return template;
  }
  return template.replaceAll(/\{(\w+)\}/g, (match, key: string) => {
    const value = params[key];
    return value === undefined ? match : String(value);
  });
}

export function readStoredLocale(initialLocale?: Locale): Locale {
  if (initialLocale) {
    return initialLocale;
  }
  if (typeof window === "undefined") {
    return defaultLocale;
  }
  const stored = window.localStorage.getItem(localeStorageKey);
  if (stored === "en" || stored === "ko") {
    return stored;
  }
  return defaultLocale;
}

function toLocaleTag(locale: Locale): string {
  return locale === "ko" ? "ko-KR" : "en-US";
}

function getCachedLocaleDictionary(
  locale: Locale,
): TranslationDictionary | null {
  return dictionaryCache[locale] ?? null;
}

export function primeLocaleDictionary(
  locale: Locale,
  dictionary: TranslationDictionary,
) {
  dictionaryCache[locale] = dictionary;
}

export async function loadLocaleDictionary(
  locale: Locale,
): Promise<TranslationDictionary> {
  const cached = getCachedLocaleDictionary(locale);
  if (cached) {
    return cached;
  }

  let dictionary: TranslationDictionary;
  switch (locale) {
    case "ko": {
      const module = await import("./i18n-dictionaries/ko");
      dictionary = module.ko;
      break;
    }
    case "en":
    default:
      dictionary = en;
      break;
  }

  dictionaryCache[locale] = dictionary;
  return dictionary;
}

export function I18nProvider({
  children,
  initialLocale,
}: PropsWithChildren<{ initialLocale?: Locale }>) {
  const resolvedInitialLocale = readStoredLocale(initialLocale);
  const [locale, setLocaleState] = useState<Locale>(resolvedInitialLocale);
  const [dictionary, setDictionary] = useState<TranslationDictionary>(
    () => getCachedLocaleDictionary(resolvedInitialLocale) ?? en,
  );

  useEffect(() => {
    if (typeof window === "undefined" || initialLocale) {
      return;
    }
    window.localStorage.setItem(localeStorageKey, locale);
  }, [initialLocale, locale]);

  useEffect(() => {
    let cancelled = false;
    void loadLocaleDictionary(locale).then((nextDictionary) => {
      if (cancelled) {
        return;
      }
      setDictionary(nextDictionary);
    });
    return () => {
      cancelled = true;
    };
  }, [locale]);

  const setLocale = useCallback((next: Locale) => {
    if (next === locale) {
      return;
    }

    const cached = getCachedLocaleDictionary(next);
    if (cached) {
      startTransition(() => {
        setDictionary(cached);
        setLocaleState(next);
      });
      return;
    }

    void loadLocaleDictionary(next).then((nextDictionary) => {
      startTransition(() => {
        setDictionary(nextDictionary);
        setLocaleState(next);
      });
    });
  }, [locale]);

  const value = useMemo<I18nContextValue>(() => {
    const localeTag = toLocaleTag(locale);
    return {
      formatDateTime: (
        input: Date | number,
        options: Intl.DateTimeFormatOptions = {
          dateStyle: "medium",
          timeStyle: "short",
        },
      ) => new Intl.DateTimeFormat(localeTag, options).format(input),
      formatNumber: (
        input: bigint | number,
        options?: Intl.NumberFormatOptions,
      ) => new Intl.NumberFormat(localeTag, options).format(input),
      locale,
      localeTag,
      setLocale,
      t: (key, params) => interpolate(dictionary[key] ?? en[key], params),
    };
  }, [dictionary, locale, setLocale]);

  return createElement(I18nContext.Provider, { value }, children);
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used inside an I18nProvider");
  }
  return value;
}
