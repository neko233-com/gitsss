// Adapted from React Bits AnimatedContent (MIT + Commons Clause).
// https://github.com/DavidHDev/react-bits

import { useEffect, useRef, type HTMLAttributes, type ReactNode } from "react"

import { cn } from "@/lib/utils"

interface AnimatedContentProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
  container?: Element | string | null
  distance?: number
  direction?: "vertical" | "horizontal"
  reverse?: boolean
  duration?: number
  ease?: string
  initialOpacity?: number
  animateOpacity?: boolean
  scale?: number
  threshold?: number
  delay?: number
  disappearAfter?: number
  disappearDuration?: number
  disappearEase?: string
  onComplete?: () => void
  onDisappearanceComplete?: () => void
}

export function AnimatedContent({
  children,
  container,
  distance = 100,
  direction = "vertical",
  reverse = false,
  duration = 0.8,
  ease = "power3.out",
  initialOpacity = 0,
  animateOpacity = true,
  scale = 1,
  threshold = 0.1,
  delay = 0,
  disappearAfter = 0,
  disappearDuration = 0.5,
  disappearEase = "power3.in",
  onComplete,
  onDisappearanceComplete,
  className,
  ...props
}: AnimatedContentProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const element = ref.current
    if (!element) return

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      element.style.opacity = "1"
      element.style.transform = "none"
      element.style.visibility = "visible"
      onComplete?.()
      return
    }

    let disposed = false
    let teardown: (() => void) | undefined
    void Promise.all([import("gsap"), import("gsap/ScrollTrigger")])
      .then(([{ gsap }, { ScrollTrigger }]) => {
        if (disposed) return
        gsap.registerPlugin(ScrollTrigger)
        let scrollerTarget: Element | string | null =
          container ?? document.getElementById("snap-main-container")
        if (typeof scrollerTarget === "string") {
          scrollerTarget = document.querySelector(scrollerTarget)
        }

        const axis = direction === "horizontal" ? "x" : "y"
        const offset = reverse ? -distance : distance
        const startPercentage = (1 - threshold) * 100
        let disappearTween: ReturnType<typeof gsap.to> | undefined

        gsap.set(element, {
          [axis]: offset,
          scale,
          opacity: animateOpacity ? initialOpacity : 1,
          visibility: "visible",
        })

        const timeline = gsap.timeline({
          paused: true,
          delay,
          onComplete: () => {
            onComplete?.()
            if (disappearAfter <= 0) return
            disappearTween = gsap.to(element, {
              [axis]: reverse ? distance : -distance,
              scale: 0.8,
              opacity: animateOpacity ? initialOpacity : 0,
              delay: disappearAfter,
              duration: disappearDuration,
              ease: disappearEase,
              onComplete: onDisappearanceComplete,
            })
          },
        })

        timeline.to(element, {
          [axis]: 0,
          scale: 1,
          opacity: 1,
          duration,
          ease,
        })

        const trigger = ScrollTrigger.create({
          trigger: element,
          scroller: scrollerTarget ?? window,
          start: `top ${startPercentage}%`,
          once: true,
          onEnter: () => timeline.play(),
        })
        teardown = () => {
          disappearTween?.kill()
          trigger.kill()
          timeline.kill()
        }
      })
      .catch(() => {
        if (disposed) return
        element.style.opacity = "1"
        element.style.transform = "none"
        element.style.visibility = "visible"
      })

    return () => {
      disposed = true
      teardown?.()
    }
  }, [
    animateOpacity,
    container,
    delay,
    direction,
    disappearAfter,
    disappearDuration,
    disappearEase,
    distance,
    duration,
    ease,
    initialOpacity,
    onComplete,
    onDisappearanceComplete,
    reverse,
    scale,
    threshold,
  ])

  return (
    <div className={cn("invisible", className)} ref={ref} {...props}>
      {children}
    </div>
  )
}
