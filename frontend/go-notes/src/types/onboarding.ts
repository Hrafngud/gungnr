export type OnboardingLink = {
  label: string
  href: string
}

export type OnboardingStep = {
  id: string
  title: string
  description: string
  target?: string
  links?: OnboardingLink[]
  hint?: string
}
