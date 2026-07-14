import type { SVGProps } from "react"

type IconProps = SVGProps<SVGSVGElement>

function IconFrame({ children, ...props }: IconProps) {
  return (
    <svg
      aria-hidden="true"
      fill="none"
      stroke="currentColor"
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth="1.65"
      viewBox="0 0 24 24"
      {...props}
    >
      {children}
    </svg>
  )
}

export function BrandMark(props: IconProps) {
  return (
    <svg aria-hidden="true" fill="currentColor" viewBox="0 0 24 24" {...props}>
      <circle cx="7.2" cy="7.2" r="3.5" />
      <circle cx="16.8" cy="7.2" r="3.5" />
      <circle cx="7.2" cy="16.8" r="3.5" />
      <circle cx="16.8" cy="16.8" r="3.5" />
    </svg>
  )
}

export function OverviewIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <rect x="4" y="4" width="6" height="6" rx="1.25" />
      <rect x="14" y="4" width="6" height="6" rx="1.25" />
      <rect x="4" y="14" width="6" height="6" rx="1.25" />
      <rect x="14" y="14" width="6" height="6" rx="1.25" />
    </IconFrame>
  )
}

export function WorkIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <path d="M7 3.75h10l2 2v14.5H5V5.75l2-2Z" />
      <path d="m8.25 12.25 2.2 2.2 4.8-5" />
    </IconFrame>
  )
}

export function RepositoryIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="7" cy="5.5" r="2" />
      <circle cx="17" cy="18.5" r="2" />
      <circle cx="7" cy="18.5" r="2" />
      <path d="M7 7.5v7a4 4 0 0 0 4 4h4" />
    </IconFrame>
  )
}

export function SettingsIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="12" cy="12" r="3" />
      <path d="M18.7 14.75a7.3 7.3 0 0 0 .05-1.5l1.75-1.2-1.75-3.03-2.05.8a7.44 7.44 0 0 0-1.3-.75L15.1 6.9H11.6l-.3 2.17c-.46.2-.9.45-1.3.75l-2.05-.8L6.2 12.05l1.75 1.2a7.3 7.3 0 0 0 .05 1.5l-1.75 1.2 1.75 3.03 2.05-.8c.4.3.84.55 1.3.75l.3 2.17h3.5l.3-2.17c.46-.2.9-.45 1.3-.75l2.05.8 1.75-3.03-1.75-1.2Z" />
    </IconFrame>
  )
}

export function SearchIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="10.5" cy="10.5" r="5.25" />
      <path d="m15 15 4.1 4.1" />
    </IconFrame>
  )
}

export function ClockIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="12" cy="12" r="8.25" />
      <path d="M12 7.75v4.7l3.1 1.75" />
    </IconFrame>
  )
}

export function ArrowUpIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <path d="m12 18.5 0-13" />
      <path d="m6.75 10 5.25-5.25L17.25 10" />
    </IconFrame>
  )
}

export function AlertIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="12" cy="12" r="8.25" />
      <path d="M12 7.5v5" />
      <path d="M12 16.25h.01" />
    </IconFrame>
  )
}

export function CloseIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <path d="m7 7 10 10M17 7 7 17" />
    </IconFrame>
  )
}

export function CheckIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <path d="m5.5 12.25 4.05 4.05L18.5 7.75" />
    </IconFrame>
  )
}

export function MoreIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <circle cx="12" cy="5" r=".8" fill="currentColor" />
      <circle cx="12" cy="12" r=".8" fill="currentColor" />
      <circle cx="12" cy="19" r=".8" fill="currentColor" />
    </IconFrame>
  )
}

export function StorageIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <ellipse cx="12" cy="6.25" rx="6.75" ry="2.75" />
      <path d="M5.25 6.25v6.25c0 1.52 3.02 2.75 6.75 2.75s6.75-1.23 6.75-2.75V6.25" />
      <path d="M5.25 12.5v5.25c0 1.52 3.02 2.75 6.75 2.75s6.75-1.23 6.75-2.75V12.5" />
    </IconFrame>
  )
}

export function DownloadIcon(props: IconProps) {
  return (
    <IconFrame {...props}>
      <path d="M12 4v10.25" />
      <path d="m7.75 10 4.25 4.25L16.25 10" />
      <path d="M5.25 19.25h13.5" />
    </IconFrame>
  )
}
