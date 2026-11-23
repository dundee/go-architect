export type PackageLayer = 'internal' | 'organization' | 'external' | 'standard'

export interface LayerMeta {
  id: PackageLayer
  label: string
  shortLabel: string
  dotColor: string
  badgeClass: string
  nodeBg: string
  nodeBorder: string
  nodeText: string
  cellBg: string
}

export const LAYER_ORDER: PackageLayer[] = [
  'internal',
  'organization',
  'external',
  'standard',
]

export const LAYER_META: Record<PackageLayer, LayerMeta> = {
  internal: {
    id: 'internal',
    label: 'Internal Packages',
    shortLabel: 'Internal',
    dotColor: '#0284c7',
    badgeClass: 'bg-sky-50 text-sky-700 ring-1 ring-inset ring-sky-600/20',
    nodeBg: '#f0f9ff',
    nodeBorder: '#38bdf8',
    nodeText: '#0c4a6e',
    cellBg: 'bg-sky-50/70',
  },
  organization: {
    id: 'organization',
    label: 'Organization Packages',
    shortLabel: 'Organization',
    dotColor: '#d97706',
    badgeClass: 'bg-amber-50 text-amber-700 ring-1 ring-inset ring-amber-600/20',
    nodeBg: '#fffbeb',
    nodeBorder: '#fbbf24',
    nodeText: '#78350f',
    cellBg: 'bg-amber-50/70',
  },
  external: {
    id: 'external',
    label: 'External Packages',
    shortLabel: 'External',
    dotColor: '#e11d48',
    badgeClass: 'bg-rose-50 text-rose-700 ring-1 ring-inset ring-rose-600/20',
    nodeBg: '#fff1f2',
    nodeBorder: '#fb7185',
    nodeText: '#881337',
    cellBg: 'bg-rose-50/70',
  },
  standard: {
    id: 'standard',
    label: 'Standard Library',
    shortLabel: 'Standard',
    dotColor: '#65a30d',
    badgeClass: 'bg-lime-50 text-lime-700 ring-1 ring-inset ring-lime-600/20',
    nodeBg: '#f7fee7',
    nodeBorder: '#a3e635',
    nodeText: '#365314',
    cellBg: 'bg-lime-50/70',
  },
}

export function resolvePackageLayer(
  packageName: string,
  mainPackage: string,
  orgPackages: string[] = [],
): PackageLayer {
  if (
    mainPackage &&
    (packageName === mainPackage || packageName.startsWith(`${mainPackage}/`))
  ) {
    return 'internal'
  }
  if (orgPackages.some((prefix) => prefix && packageName.startsWith(prefix))) {
    return 'organization'
  }
  if (packageName.startsWith('golang.org/x') || !packageName.includes('.')) {
    return 'standard'
  }
  return 'external'
}

export function shortenPackageName(
  packageName: string,
  mainPackage: string,
): string {
  if (!mainPackage) return packageName
  if (packageName === mainPackage) return '(root)'
  const prefix = mainPackage.endsWith('/') ? mainPackage : `${mainPackage}/`
  if (packageName.startsWith(prefix)) {
    return packageName.slice(prefix.length)
  }
  return packageName
}
