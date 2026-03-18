import { Globe, KeyRound, Cloud, Container } from 'lucide-react'
import { cn } from '@/lib/utils'

const icons: Record<string, React.ElementType> = {
  http: Globe,
  ssh: KeyRound,
  ssm: Cloud,
  k8s: Container,
}

interface TargetIconProps {
  method: string
  className?: string
  size?: number
}

export function TargetIcon({ method, className, size = 14 }: TargetIconProps) {
  const Icon = icons[method] || Globe
  return <Icon size={size} className={cn('text-muted-foreground', className)} />
}
