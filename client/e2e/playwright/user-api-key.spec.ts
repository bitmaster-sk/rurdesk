import { APIRequestContext, expect, Page, test } from '@playwright/test';
import { Interaction } from './support/interaction';
import { createUser } from './support/user';

const SETTINGS_URL = '/user/settings';

// Each test gets its own user, so the key list starts empty and the tests can run
// in any order without inheriting keys from each other.
async function openSettingsAsFreshUser(
    page: Page,
    request: APIRequestContext,
    baseURL: string,
    label: string
): Promise<void> {
    const user = await createUser(request, baseURL, `apikey-${label}`);
    await Interaction.login(page, user);
    await page.goto(SETTINGS_URL);
    await expect(page.getByTestId('user-api-key-count')).toBeVisible();
}

test.describe('user api keys', () => {
    // Scenario:
    // - open user settings as a user with no keys and assert the empty state, not a table
    // - create a key named "laptop"
    // - assert the raw value is revealed once and looks like a 64-character hex key
    // - assert the key appears as a table row and the raw value is NOT in the table
    // - assert the counter moved to 1
    test('creates a key and reveals its raw value exactly once', async ({
        page,
        request,
        baseURL
    }) => {
        await openSettingsAsFreshUser(page, request, baseURL!, 'create');
        await expect(page.getByTestId('user-api-key-empty')).toBeVisible();

        const rawKey = await Interaction.createUserApiKey(page, 'laptop');
        expect(rawKey).toMatch(/^[0-9a-f]{64}$/);

        const table = page.getByTestId('user-api-key-table');
        await expect(table).toContainText('laptop');
        await expect(table).not.toContainText(rawKey);
        await expect(page.getByTestId('user-api-key-count')).toContainText('1 /');
    });

    // Scenario:
    // - create two keys
    // - reload so the list is re-fetched from the server rather than kept locally
    // - assert both names are listed and exactly two rows are present
    test('lists the keys the user created', async ({ page, request, baseURL }) => {
        await openSettingsAsFreshUser(page, request, baseURL!, 'list');
        await Interaction.createUserApiKey(page, 'first');
        await Interaction.createUserApiKey(page, 'second');

        await page.reload();

        const table = page.getByTestId('user-api-key-table');
        await expect(table).toContainText('first');
        await expect(table).toContainText('second');
        await expect(page.getByTestId('user-api-key-row')).toHaveCount(2);
    });

    // Scenario:
    // - create a key and remember its raw value
    // - regenerate it and accept the confirmation
    // - assert a new raw value is revealed and differs from the original
    // - assert the row count stayed at 1 and the name is unchanged, proving rotation
    //   replaces the key in place instead of adding a second one
    test('regenerates a key in place with a new raw value', async ({ page, request, baseURL }) => {
        await openSettingsAsFreshUser(page, request, baseURL!, 'rotate');
        const original = await Interaction.createUserApiKey(page, 'rotate-me');

        await page.getByTestId('user-api-key-regenerate').click();
        await Interaction.acceptConfirm(page);

        const revealed = page.getByTestId('user-api-key-revealed');
        await expect(revealed).toBeVisible();
        await expect(revealed.locator('code')).not.toHaveText(original);
        await expect(page.getByTestId('user-api-key-row')).toHaveCount(1);
        await expect(page.getByTestId('user-api-key-table')).toContainText('rotate-me');
    });

    // Scenario:
    // - create a key and assert it is listed
    // - revoke it and accept the confirmation
    // - assert the empty state is back
    // - reload and assert it stayed gone, so the delete reached the server
    test('revokes a key and it stays gone after a reload', async ({ page, request, baseURL }) => {
        await openSettingsAsFreshUser(page, request, baseURL!, 'revoke');
        await Interaction.createUserApiKey(page, 'revoke-me');
        await expect(page.getByTestId('user-api-key-row')).toHaveCount(1);

        await page.getByTestId('user-api-key-revoke').click();
        await Interaction.acceptConfirm(page);

        await expect(page.getByTestId('user-api-key-empty')).toBeVisible();

        await page.reload();
        await expect(page.getByTestId('user-api-key-empty')).toBeVisible();
    });
});
