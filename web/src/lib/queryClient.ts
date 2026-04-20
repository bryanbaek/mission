import { createContext, useContext } from "react";
import { createClient, type Client, type Transport } from "@connectrpc/connect";

import { QueryService } from "../gen/query/v1/query_pb";

export type QueryClient = Client<typeof QueryService>;

export function createQueryClientFromTransport(
  transport: Transport,
): QueryClient {
  return createClient(QueryService, transport);
}

export const QueryClientContext = createContext<QueryClient | null>(null);

export function useQueryClient(): QueryClient {
  const client = useContext(QueryClientContext);
  if (!client) {
    throw new Error("useQueryClient must be used inside a QueryClientProvider");
  }
  return client;
}
