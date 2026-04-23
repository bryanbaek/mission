import type { ReactNode } from "react";
import { Link } from "react-router-dom";

import { useI18n } from "../../lib/i18n";
import LocaleToggle from "./LocaleToggle";
import { styles } from "./onboardingStepUtils";

export default function StepFrame({
  step,
  title,
  subtitle,
  justHappened,
  nextText,
  backHref,
  children,
}: {
  step: number;
  title: string;
  subtitle: string;
  justHappened: string;
  nextText: string;
  backHref?: string;
  children: ReactNode;
}) {
  const { t } = useI18n();
  const progress = `${(step / 7) * 100}%`;

  return (
    <div className={styles.shell}>
      <section className={styles.hero}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-slate-500">
              {t("onboarding.common.stepOfTotal", {
                step,
                total: 7,
              })}
            </p>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight text-slate-900">
              {title}
            </h1>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            {backHref ? (
              <Link to={backHref} className={styles.buttonSecondary}>
                {t("onboarding.common.back")}
              </Link>
            ) : null}
            <LocaleToggle />
          </div>
        </div>
        <div className={styles.progressTrack}>
          <div className={styles.progressFill} style={{ width: progress }} />
        </div>
        <p className="max-w-3xl text-sm leading-6 text-slate-600">{subtitle}</p>
        <div className="grid gap-4 lg:grid-cols-2">
          <div className="rounded-3xl bg-slate-50 p-4">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-400">
              {t("onboarding.common.justHappenedLabel")}
            </p>
            <p className="mt-2 text-sm leading-6 text-slate-700">
              {justHappened}
            </p>
          </div>
          <div className="rounded-3xl bg-emerald-50 p-4">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-emerald-700">
              {t("onboarding.common.nextLabel")}
            </p>
            <p className="mt-2 text-sm leading-6 text-emerald-900">
              {nextText}
            </p>
          </div>
        </div>
      </section>
      {children}
    </div>
  );
}
