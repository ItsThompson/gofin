import type { Route } from "./+types/home";
import { LandingPage, landingContent } from "@/features/marketing";

/** SEO metadata for `/`, server-rendered via root.tsx's <Meta />. */
export const meta: Route.MetaFunction = () => [
  { title: landingContent.meta.title },
  { name: "description", content: landingContent.meta.description },
];

export default function HomeRoute() {
  return <LandingPage />;
}
