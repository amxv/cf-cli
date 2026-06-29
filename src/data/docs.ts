export const siteConfig = {
  name: "cf-cli",
  strapline: "Cloudflare operations from your terminal",
  description:
    "Documentation for cf-cli, a practical Cloudflare operations CLI for DNS records, API token minting, Workers logs, R2 helpers, local profiles, Wrangler account switching, and environment diagnostics.",
  repoUrl: "https://github.com/amxv/cf-cli"
} as const;

export const docCategories = [
  "Start",
  "Cloudflare Operations",
  "Local Workflows",
  "Reference"
] as const;

export const primaryNav = [
  { href: "/", label: "Overview" },
  { href: "/docs", label: "Docs" },
  { href: siteConfig.repoUrl, label: "GitHub", external: true }
];
