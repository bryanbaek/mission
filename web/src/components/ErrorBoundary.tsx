import * as Sentry from "@sentry/react";
import { Component, type ErrorInfo, type ReactNode } from "react";

interface FallbackProps {
  error: Error;
  resetError: () => void;
}

interface Props {
  children: ReactNode;
  fallback?: (props: FallbackProps) => ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Application error boundary that reports errors to Sentry (when configured)
 * and renders a fallback UI. Wrap this around route trees or individual pages
 * to prevent a single component error from crashing the entire app.
 */
export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { error: null };
  }

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  override componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    Sentry.captureException(error, {
      extra: { componentStack: errorInfo.componentStack },
    });
  }

  resetError = () => {
    this.setState({ error: null });
  };

  override render() {
    const { error } = this.state;
    if (error) {
      if (this.props.fallback) {
        return this.props.fallback({ error, resetError: this.resetError });
      }
      return (
        <div className="mx-auto max-w-lg p-8 text-center">
          <h2 className="mb-2 text-lg font-semibold text-slate-900">
            Something went wrong
          </h2>
          <p className="mb-4 text-sm text-slate-600">{error.message}</p>
          <button
            type="button"
            onClick={this.resetError}
            className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50"
          >
            Try again
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
