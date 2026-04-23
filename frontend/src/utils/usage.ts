export function getUsageClass(value: number): string {
  if (value < 60) return 'usage-low'
  if (value < 80) return 'usage-medium'
  return 'usage-high'
}

export function perCoreUsage(cpuPercent: number, cpuCores?: number): number {
  return cpuPercent / (cpuCores || 1)
}
