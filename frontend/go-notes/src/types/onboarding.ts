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

export type OnboardingState = {
  home: boolean
  hostSettings: boolean
  networking: boolean
  github: boolean
}

export type OnboardingUpdate = Partial<OnboardingState>
