export interface ServicePreset {
  id: string;
  name: string;
  subdomain: string;
  port: number;
  image: string;
  containerPort: number;
  description: string;
  repoLabel: string;
  repoUrl: string;
  icon?: string; // Icon identifier for the service type
}