export type RuntimeConfig = {
  clerkPublishableKey?: string;
  sentryDsn?: string;
  sentryEnvironment?: string;
  sentryRelease?: string;
};

let runtimeConfig: RuntimeConfig = {};

export async function loadRuntimeConfig(): Promise<RuntimeConfig> {
  try {
    const response = await fetch("/app-config.json", { cache: "no-store" });
    if (!response.ok) {
      return runtimeConfig;
    }
    runtimeConfig = (await response.json()) as RuntimeConfig;
  } catch {
    return runtimeConfig;
  }
  return runtimeConfig;
}

export function getRuntimeConfig(): RuntimeConfig {
  return runtimeConfig;
}
