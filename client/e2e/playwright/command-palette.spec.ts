import { test, expect } from '@playwright/test';

// Frontend-only command-palette behaviour — no backend needed. The HotkeyService is started
// from AppComponent's constructor, so the global ⌘K / bare `/` / `?` shortcuts work on the
// login shell even before any project loads (the palette simply shows no commands there).
test.describe('command palette', () => {
    test('⌘/Ctrl+K opens the palette and Escape closes it', async ({ page }) => {
        await page.goto('/login');

        // The mod+K path fires regardless of focus (even inside the email field).
        await page.keyboard.press('Control+k');
        const palette = page.locator('.palette');
        await expect(palette).toBeVisible();
        await expect(palette.locator('input')).toBeFocused();

        // Escape on an empty query closes the palette.
        await page.keyboard.press('Escape');
        await expect(palette).toHaveCount(0);
    });

    test('bare "/" opens the palette in navigation mode', async ({ page }) => {
        await page.goto('/login');
        // Move focus off any autofocused input so the bare-key gate lets `/` through.
        await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());

        await page.keyboard.press('/');
        const palette = page.locator('.palette');
        await expect(palette).toBeVisible();
        // Mode chip reflects the navigation scope.
        await expect(palette.locator('.palette__mode')).toContainText(/navigat/i);

        // Opened via `/`, the input carries the "/" prefix — Escape clears it first, then closes
        // (design: Escape clears a non-empty query before closing).
        await page.keyboard.press('Escape');
        await page.keyboard.press('Escape');
        await expect(palette).toHaveCount(0);
    });

    test('"?" opens the keyboard-shortcuts help sheet', async ({ page }) => {
        await page.goto('/login');
        // Blur any autofocused field so the bare-key gate lets `?` through.
        await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());

        await page.keyboard.press('?');
        const help = page.locator('[data-help]');
        await expect(help).toBeVisible();
        // The sheet lists shortcut keys.
        await expect(help.locator('kbd').first()).toBeVisible();

        await page.keyboard.press('Escape');
        await expect(help).toHaveCount(0);
    });
});
