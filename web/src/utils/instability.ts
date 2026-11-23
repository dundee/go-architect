import type { PackageInstability } from '../types/api'

export type InstabilityZone = 'healthy' | 'warning' | 'pain' | 'useless'

export interface ZoneMeta {
  id: InstabilityZone
  label: string
  shortLabel: string
  description: string
  badgeClass: string
  dotFill: string
  dotStroke: string
}

export const ZONE_META: Record<InstabilityZone, ZoneMeta> = {
  healthy: {
    id: 'healthy',
    label: 'Main Sequence',
    shortLabel: 'Balanced',
    description: 'Well balanced between abstractness and stability (D <= 0.2).',
    badgeClass: 'bg-emerald-50 text-emerald-700 ring-1 ring-inset ring-emerald-600/20',
    dotFill: '#059669',
    dotStroke: '#ecfdf5',
  },
  warning: {
    id: 'warning',
    label: 'Buffer Zone',
    shortLabel: 'Moderate Drift',
    description: 'Moderate distance from the main sequence (0.2 < D < 0.4).',
    badgeClass: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
    dotFill: '#d97706',
    dotStroke: '#fffbeb',
  },
  pain: {
    id: 'pain',
    label: 'Zone of Pain',
    shortLabel: 'Zone of Pain',
    description: 'Concrete & rigid: heavily depended upon with few abstractions (D >= 0.4, A + I < 1).',
    badgeClass: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
    dotFill: '#e11d48',
    dotStroke: '#fff1f2',
  },
  useless: {
    id: 'useless',
    label: 'Zone of Uselessness',
    shortLabel: 'Zone of Uselessness',
    description: 'Abstract & unreferenced: abstractions nobody depends on (D >= 0.4, A + I > 1).',
    badgeClass: 'bg-orange-50 text-orange-700 ring-1 ring-inset ring-orange-600/20',
    dotFill: '#ea580c',
    dotStroke: '#fff7ed',
  },
}

export function classifyZone(pkg: PackageInstability): InstabilityZone {
  if (pkg.distance <= 0.2) {
    return 'healthy'
  }
  if (pkg.distance < 0.4) {
    return 'warning'
  }
  if (pkg.abstractness + pkg.instability < 1) {
    return 'pain'
  }
  return 'useless'
}

export interface PointGroup {
  key: string
  instability: number
  abstractness: number
  distance: number
  zone: InstabilityZone
  packages: PackageInstability[]
}

export function coordinateKey(instability: number, abstractness: number): string {
  return `${instability.toFixed(4)}:${abstractness.toFixed(4)}`
}

export function groupPackagesByCoordinate(
  packages: PackageInstability[],
): PointGroup[] {
  const map = new Map<string, PointGroup>()

  for (const pkg of packages) {
    const key = coordinateKey(pkg.instability, pkg.abstractness)
    const existing = map.get(key)
    if (existing) {
      existing.packages.push(pkg)
    } else {
      map.set(key, {
        key,
        instability: pkg.instability,
        abstractness: pkg.abstractness,
        distance: pkg.distance,
        zone: classifyZone(pkg),
        packages: [pkg],
      })
    }
  }

  return Array.from(map.values())
}

export function formatPackageLabel(
  packageName: string,
  modulePath?: string,
): { display: string; full: string } {
  if (!modulePath) {
    return { display: packageName, full: packageName }
  }
  if (packageName === modulePath) {
    return { display: '(root)', full: packageName }
  }
  const prefix = modulePath.endsWith('/') ? modulePath : `${modulePath}/`
  if (packageName.startsWith(prefix)) {
    return { display: packageName.slice(prefix.length), full: packageName }
  }
  return { display: packageName, full: packageName }
}

export function formatMetric(value: number): string {
  return Number.isInteger(value) ? value.toFixed(1) : value.toFixed(2)
}
