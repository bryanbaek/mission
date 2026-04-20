export const onboardingStepCount = 7;

export function clampOnboardingStep(step: number): number {
  if (!Number.isFinite(step)) {
    return 1;
  }
  return Math.max(1, Math.min(onboardingStepCount, Math.trunc(step)));
}

export function onboardingStepPath(tenantId: string, step: number): string {
  return `/onboarding/${tenantId}/step-${clampOnboardingStep(step)}`;
}
