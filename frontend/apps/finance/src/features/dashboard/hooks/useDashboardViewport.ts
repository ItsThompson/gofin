import { useEffect, useState } from "react";

function isDesktopViewport(): boolean {
  return typeof window === "undefined" || !window.matchMedia || window.matchMedia("(min-width: 768px)").matches;
}

export function useDashboardViewport(): boolean {
  const [desktopVisible, setDesktopVisible] = useState(isDesktopViewport);

  useEffect(() => {
    setDesktopVisible(isDesktopViewport());
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mediaQuery = window.matchMedia("(min-width: 768px)");
    const onChange = (event: MediaQueryListEvent) => setDesktopVisible(event.matches);
    mediaQuery.addEventListener("change", onChange);
    return () => mediaQuery.removeEventListener("change", onChange);
  }, []);

  return desktopVisible;
}
