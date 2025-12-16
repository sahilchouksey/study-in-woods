import type { NextConfig } from "next";
import path from "path";

const nextConfig: NextConfig = {
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
