import type { NextConfig } from "next";

const internalBase = (
  process.env.API_INTERNAL_BASE_URL || "http://127.0.0.1:8080/api/v1"
).replace(/\/api\/v1\/?$/, "");

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
];

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  eslint: { ignoreDuringBuilds: false },
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: securityHeaders,
      },
    ];
  },
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${internalBase}/api/:path*` },
      { source: "/files/:path*", destination: `${internalBase}/files/:path*` },
    ];
  },
};

export default nextConfig;
