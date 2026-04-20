import { createContext, useContext } from "react";
import { createClient, type Client, type Transport } from "@connectrpc/connect";

import { StarterQuestionsService } from "../gen/starter/v1/starter_pb";

export type StarterQuestionsClient = Client<typeof StarterQuestionsService>;

export function createStarterQuestionsClientFromTransport(
  transport: Transport,
): StarterQuestionsClient {
  return createClient(StarterQuestionsService, transport);
}

export const StarterQuestionsClientContext =
  createContext<StarterQuestionsClient | null>(null);

export function useStarterQuestionsClient(): StarterQuestionsClient {
  const client = useContext(StarterQuestionsClientContext);
  if (!client) {
    throw new Error(
      "useStarterQuestionsClient must be used inside a StarterQuestionsClientProvider",
    );
  }
  return client;
}
