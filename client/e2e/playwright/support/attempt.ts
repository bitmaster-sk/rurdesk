import { test } from '@playwright/test';

export abstract class Attempt {
    // Scopes a label to the current attempt. A retry must not reuse what its failed attempt
    // already created: a user who owns a project never sees the onboarding form again, and
    // the stub gateway still reports the merged PR of the previous attempt. Applying this
    // twice to the same label must stay a no-op, because helpers call it on their own.
    public static label(label: string): string {
        const { retry } = test.info();
        const suffix = `-retry${retry}`;
        if (retry === 0 || label.endsWith(suffix)) {
            return label;
        }
        return `${label}${suffix}`;
    }
}
