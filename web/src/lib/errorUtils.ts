import { ConnectError } from "@connectrpc/connect";

/**
 * Extracts a human-readable error message from any thrown value.
 * For Connect-RPC errors, returns the raw message without the gRPC status prefix.
 * For standard Error instances, returns the message property.
 * For anything else, returns a generic fallback.
 */
export function errorMessage(err: unknown): string {
  if (err instanceof ConnectError) {
    return err.rawMessage;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
