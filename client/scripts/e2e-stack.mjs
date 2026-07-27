// Runs the Playwright e2e suite against a FRESH, throwaway stack.
//
// The suite needs a virgin instance: onboarding.spec.ts registers the bootstrap
// (first) user, and public registration closes for good once any user exists.
// Pointing it at the shared dev stack therefore fails — see ensureBootstrapUser.
// This script owns the whole lifecycle so nobody has to remember that: bring the
// isolated stack up, wait for it to serve, run the tests, always tear it down
// (including volumes, so the next run starts virgin again).
//
// Extra CLI args are forwarded to Playwright: npm run e2e:stack -- --headed
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const clientDir = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const composeFile = resolve(clientDir, '../docker/e2e/docker-compose.e2e.yml');
const baseURL = 'http://localhost:1000';
const readyTimeoutMs = 120_000;

const compose = (...args) =>
    spawnSync('docker', ['compose', '-f', composeFile, ...args], {
        stdio: 'inherit',
        cwd: clientDir
    });

/** Polls the SPA until the tracker serves it, so tests never race the boot. */
async function waitForStack() {
    const deadline = Date.now() + readyTimeoutMs;
    while (Date.now() < deadline) {
        try {
            const res = await fetch(`${baseURL}/login`);
            if (res.ok) return;
        } catch {
            // Connection refused while the container boots — keep polling.
        }
        await new Promise(r => setTimeout(r, 1000));
    }
    throw new Error(`stack did not serve ${baseURL}/login within ${readyTimeoutMs / 1000}s`);
}

let exitCode = 1;
try {
    console.log('> bringing up the isolated e2e stack');
    const up = compose('up', '-d', '--build');
    if (up.status !== 0) throw new Error('docker compose up failed');

    await waitForStack();

    console.log(`> running Playwright against ${baseURL}`);
    const test = spawnSync('npx', ['playwright', 'test', ...process.argv.slice(2)], {
        stdio: 'inherit',
        cwd: clientDir,
        env: { ...process.env, E2E_BASE_URL: baseURL }
    });
    exitCode = test.status ?? 1;
} catch (err) {
    console.error(`e2e stack run failed: ${err.message}`);
} finally {
    // Always tear down, including on failure — a surviving stack would keep port
    // 1000 busy and let the next run start from a non-virgin database.
    console.log('> tearing down the e2e stack');
    compose('down', '-v');
}

process.exit(exitCode);
