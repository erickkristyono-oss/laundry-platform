import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Lean standalone server output for the Docker image
  // (deployments/docker/Dockerfile.web) — bundles only the traced
  // dependencies instead of the full node_modules tree.
  output: "standalone",
};

export default nextConfig;
