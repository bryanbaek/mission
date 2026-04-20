import { createContext, useContext } from "react";
import { createClient, type Client, type Transport } from "@connectrpc/connect";

import { SemanticLayerService } from "../gen/semantic/v1/semantic_pb";

export type SemanticClient = Client<typeof SemanticLayerService>;

export function createSemanticClientFromTransport(
  transport: Transport,
): SemanticClient {
  return createClient(SemanticLayerService, transport);
}

export const SemanticClientContext = createContext<SemanticClient | null>(null);

export function useSemanticClient(): SemanticClient {
  const client = useContext(SemanticClientContext);
  if (!client) {
    throw new Error(
      "useSemanticClient must be used inside a SemanticClientProvider",
    );
  }
  return client;
}
