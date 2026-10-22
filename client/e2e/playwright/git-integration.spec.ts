import { test, expect } from '@playwright/test';
import { createUser } from './support/user';
import { Interaction } from './support/interaction';
import { StubGitIntegration } from './support/agent';

// Scenario:
// - create a dedicated non-admin user and log in
// - create a blank project and open its settings
// - assert New integration is enabled while the project has no integration
// - add the project's git integration through the API
// - reload the settings and assert the integration is listed and New integration is disabled
test('a project accepts only one git integration', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'git-integration-limit');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Git Integration Limit');

    await page.goto(`/project/${idProject}/settings`);
    const addButton = page.locator('[data-testid="git-integration-add"] button');
    await expect(addButton).toBeEnabled();

    const token = await Interaction.apiToken(request, baseURL!, user);
    await StubGitIntegration.ensure(request, baseURL!, token, idProject);

    await page.reload();
    await expect(page.getByText(`e2e-${idProject}`)).toBeVisible();
    await expect(addButton).toBeDisabled();
});
