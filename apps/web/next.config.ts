import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
  // Enable standalone output for Docker deployment
  output: 'standalone',
  // Ignore ESLint errors during build
  eslint: {
    ignoreDuringBuilds: true,
  },
  // Ignore TypeScript errors during build
  typescript: {
    ignoreBuildErrors: true,
  },
  // Disable CSR bailout error for missing Suspense
  experimental: {
    missingSuspenseWithCSRBailout: false,
  },
  turbopack: {
    root: path.resolve(__dirname, '../../'),
  },
  // Required for Streamdown/Shiki syntax highlighting
  transpilePackages: ['shiki'],
  // Handle Mermaid Node.js dependencies
  serverExternalPackages: ['langium', '@mermaid-js/parser'],
  webpack: (config, { isServer }) => {
    if (!isServer) {
      config.resolve.alias = {
        ...config.resolve.alias,
        'vscode-jsonrpc': false,
        'langium': false,
      };
    }
    return config;
  },
};

export default nextConfig;
