import { createContext, useContext } from "react";
import {
  createClient,
  type Client,
  type Interceptor,
  type Transport,
} from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { TenantService } from "../gen/tenant/v1/tenant_pb";

export type TenantClient = Client<typeof TenantService>;

// TokenGetter returns the current session JWT, or null when unauthenticated.
export type TokenGetter = () => Promise<string | null>;

export function createAuthedTransport(getToken: TokenGetter): Transport {
  const authInterceptor: Interceptor = (next) => async (req) => {
    const token = await getToken();
    if (token) {
      req.header.set("Authorization", `Bearer ${token}`);
    }
    return next(req);
  };

  return createConnectTransport({
    baseUrl: "/",
    interceptors: [authInterceptor],
  });
}

export function createTenantClientFromTransport(
  transport: Transport,
): TenantClient {
  return createClient(TenantService, transport);
}

export function createTenantClient(getToken: TokenGetter): TenantClient {
  return createTenantClientFromTransport(createAuthedTransport(getToken));
}

// Context lets page components consume a client without pulling Clerk in
// directly. Tests provide a fake client by wrapping the page in a Provider.
export const TenantClientContext = createContext<TenantClient | null>(null);

export function useTenantClient(): TenantClient {
  const client = useContext(TenantClientContext);
  if (!client) {
    throw new Error(
      "useTenantClient must be used inside a TenantClientProvider",
    );
  }
  return client;
}
