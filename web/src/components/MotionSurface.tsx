import type { ComponentProps } from "react"

import { AnimatedContent } from "@/components/AnimatedContent"
import { cn } from "@/lib/utils"

type MotionSurfaceProps = Omit<
  ComponentProps<typeof AnimatedContent>,
  "distance" | "duration" | "ease" | "threshold"
>

export function MotionSurface({ className, ...props }: MotionSurfaceProps) {
  return (
    <AnimatedContent
      className={cn("min-w-0", className)}
      data-motion-source="react-bits"
      distance={12}
      duration={0.36}
      ease="power2.out"
      threshold={0.02}
      {...props}
    />
  )
}
