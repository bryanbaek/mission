import OnboardingStepScreen from "../components/onboarding/OnboardingStepScreen";

export default function OnboardingStepPage({ step }: { step: number }) {
  return <OnboardingStepScreen step={step} />;
}
