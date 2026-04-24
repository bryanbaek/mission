import type { ReactNode } from "react";

interface ErrorBannerProps {
  /** Simple string message. Ignored when children are provided. */
  message?: string | null | undefined;
  /** Structured content rendered inside the banner (takes precedence over message). */
  children?: ReactNode;
  className?: string;
}

/**
 * Shared error banner component used across pages and features.
 * Renders a rose-colored banner with `role="alert"`. Pass either a plain
 * `message` string or structured `children`; returns null when both are falsy.
 */
export default function ErrorBanner({
  message,
  children,
  className,
}: ErrorBannerProps) {
  const content = children ?? message;
  if (!content) return null;
  return (
    <div
      className={[
        "rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      role="alert"
    >
      {content}
    </div>
  );
}
