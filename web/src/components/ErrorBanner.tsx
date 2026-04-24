interface ErrorBannerProps {
  message: string | null | undefined;
  className?: string;
}

/**
 * Shared error banner component used across pages and features.
 * Renders a rose-colored banner with the error message, or nothing if the
 * message is falsy.
 */
export default function ErrorBanner({ message, className }: ErrorBannerProps) {
  if (!message) return null;
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
      {message}
    </div>
  );
}
