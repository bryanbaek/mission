import { useI18n } from "../../lib/i18n";

export default function LocaleToggle() {
  const { locale, setLocale, t } = useI18n();

  return (
    <div
      role="group"
      aria-label={t("common.language")}
      className="inline-flex rounded-xl bg-slate-100 p-1"
    >
      {(["en", "ko"] as const).map((nextLocale) => (
        <button
          key={nextLocale}
          type="button"
          aria-pressed={locale === nextLocale}
          onClick={() => setLocale(nextLocale)}
          className={[
            "rounded-lg px-3 py-1.5 text-xs font-semibold tracking-[0.18em]",
            locale === nextLocale
              ? "bg-slate-950 text-white"
              : "text-slate-600 hover:bg-white",
          ].join(" ")}
        >
          {nextLocale.toUpperCase()}
        </button>
      ))}
    </div>
  );
}
