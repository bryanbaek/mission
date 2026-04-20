import { createContext, useContext } from "react";
import { createClient, type Client, type Transport } from "@connectrpc/connect";

import { OnboardingService } from "../gen/onboarding/v1/onboarding_pb";

export type OnboardingClient = Client<typeof OnboardingService>;

export function createOnboardingClientFromTransport(
  transport: Transport,
): OnboardingClient {
  return createClient(OnboardingService, transport);
}

export const OnboardingClientContext = createContext<OnboardingClient | null>(
  null,
);

export function useOnboardingClient(): OnboardingClient {
  const client = useContext(OnboardingClientContext);
  if (!client) {
    throw new Error(
      "useOnboardingClient must be used inside a OnboardingClientProvider",
    );
  }
  return client;
}
