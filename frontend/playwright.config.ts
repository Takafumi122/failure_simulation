import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { defineConfig, devices } from '@playwright/test';

function resolveLocalHeadlessShell(): string | undefined {
  if (process.env.CI) return undefined;
  const cacheDir = path.join(os.homedir(), 'Library', 'Caches', 'ms-playwright');
  if (!fs.existsSync(cacheDir)) return undefined;
  try {
    const entries = fs
      .readdirSync(cacheDir)
      .filter((name) => name.startsWith('chromium_headless_shell-'))
      .sort()
      .reverse();
    for (const dir of entries) {
      const candidate = path.join(
        cacheDir,
        dir,
        'chrome-headless-shell-mac-arm64',
        'chrome-headless-shell'
      );
      if (fs.existsSync(candidate)) return candidate;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

const executablePath = resolveLocalHeadlessShell();

export default defineConfig({
  testDir: './e2e',
  timeout: 30000,
  fullyParallel: false,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:8089',
    trace: 'on-first-retry',
    launchOptions: executablePath ? { executablePath } : undefined,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'PORT=8089 STATIC_DIR=./dist ../backend/failurelab-server',
    url: 'http://localhost:8089/healthz',
    reuseExistingServer: !process.env.CI,
    timeout: 15000,
  },
});
