import { FullConfig, request } from '@playwright/test';
import { ensureBootstrapUser } from './bootstrap-user';

// Runs once, before any worker starts. Bootstrapping from inside the specs raced:
// with fullyParallel every file that needed a user called ensureBootstrapUser at
// the same moment on a virgin database, and the concurrent registrations of the
// same email made /register answer 500 instead of 200/403.
export default async function globalSetup(config: FullConfig): Promise<void> {
    const baseURL = config.projects[0].use.baseURL!;
    const context = await request.newContext({ baseURL });
    try {
        await ensureBootstrapUser(context, baseURL);
    } finally {
        await context.dispose();
    }
}
