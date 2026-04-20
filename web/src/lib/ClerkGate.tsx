import { useMemo } from "react";
import {
  ClerkProvider,
  SignedIn,
  SignedOut,
  SignIn,
  useAuth,
} from "@clerk/clerk-react";

import {
  createAuthedTransport,
  createTenantClientFromTransport,
  TenantClientContext,
  type TenantClient,
} from "./tenantClient";
import {
  createSemanticClientFromTransport,
  SemanticClientContext,
  type SemanticClient,
} from "./semanticClient";
import {
  createQueryClientFromTransport,
  QueryClientContext,
  type QueryClient,
} from "./queryClient";

type Props = {
  children: React.ReactNode;
};

// ClerkGate wraps the app with ClerkProvider, gates protected routes behind
// a sign-in screen, and exposes a Clerk-backed TenantClient to descendants.
export function ClerkGate({ children }: Props) {
  const key = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;
  if (!key) {
    return (
      <div className="mx-auto max-w-xl p-8 text-sm text-rose-700">
        Missing <code>VITE_CLERK_PUBLISHABLE_KEY</code>. Set it in{" "}
        <code>web/.env.local</code> or the deploy environment.
      </div>
    );
  }

  return (
    <ClerkProvider publishableKey={key}>
      <SignedIn>
        <AuthedClientProvider>{children}</AuthedClientProvider>
      </SignedIn>
      <SignedOut>
        <div
          className={[
            "flex min-h-screen items-center justify-center",
            "bg-slate-100 p-6",
          ].join(" ")}
        >
          <SignIn routing="hash" />
        </div>
      </SignedOut>
    </ClerkProvider>
  );
}

function AuthedClientProvider({ children }: Props) {
  const { getToken } = useAuth();
  const transport = useMemo(
    () => createAuthedTransport(() => getToken()),
    [getToken],
  );
  const client: TenantClient = useMemo(
    () => createTenantClientFromTransport(transport),
    [transport],
  );
  const semanticClient: SemanticClient = useMemo(
    () => createSemanticClientFromTransport(transport),
    [transport],
  );
  const queryClient: QueryClient = useMemo(
    () => createQueryClientFromTransport(transport),
    [transport],
  );
  return (
    <TenantClientContext.Provider value={client}>
      <SemanticClientContext.Provider value={semanticClient}>
        <QueryClientContext.Provider value={queryClient}>
          {children}
        </QueryClientContext.Provider>
      </SemanticClientContext.Provider>
    </TenantClientContext.Provider>
  );
}
