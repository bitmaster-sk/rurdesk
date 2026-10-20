import { APIRequestContext, expect, test, type Page } from '@playwright/test';
import { Interaction } from './support/interaction';
import { createUser } from './support/user';
import { authHeaders } from './support/auth';
import { BOOTSTRAP_USER } from './support/bootstrap-user';

const BLOCK = '```plantuml\nAlice -> Bob: hi\n```';

async function adminToken(request: APIRequestContext, baseURL: string): Promise<string> {
    const login = await request.post(`${baseURL}/api/public/login`, {
        data: { email: BOOTSTRAP_USER.email, password: BOOTSTRAP_USER.password }
    });
    expect(login.status()).toBe(200);
    return ((await login.json()) as { token: string }).token;
}

async function setPlantUmlUrl(request: APIRequestContext, baseURL: string, url: string) {
    const res = await request.patch(`${baseURL}/api/private/admin/settings`, {
        headers: authHeaders(await adminToken(request, baseURL)),
        data: { plantUmlServerUrl: url }
    });
    expect(res.status(), await res.text()).toBe(200);
}

async function commentOnIssue(page: Page, idProject: number, idIssue: number, body: string) {
    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const comment = page.locator('.feed-editor [contenteditable="true"]');
    await comment.click();
    await comment.fill(body);
    await comment.press('ControlOrMeta+Enter');
}

// Scenario:
// - fresh stack has no plantuml server configured (setting default is empty)
// - a wiki page with a ```plantuml block keeps the markdown code fence
// - the same block in a task comment renders the component's disabled code block
test('a plantuml block stays a code block while no server is configured', async ({
    page,
    request,
    baseURL
}) => {
    await setPlantUmlUrl(request, baseURL!, '');
    const user = await createUser(request, baseURL!, 'plantuml-off');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'PlantUML Off');

    await Interaction.createWikiPage(page, idProject, 'Off flow', `Diagram:\n\n${BLOCK}`);
    await expect(
        page.getByTestId('wiki-page-body').locator('pre > code.language-plantuml')
    ).toBeVisible();

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: 'Off task',
        description: 'No diagram.',
        state: 'New',
        severity: 'Medium'
    });
    await commentOnIssue(page, idProject, idIssue, `Diagram:\n${BLOCK}`);
    await expect(page.getByTestId('plantuml-disabled')).toBeVisible();
    await expect(page.getByTestId('plantuml-diagram-img')).toHaveCount(0);
});

// Scenario:
// - admin sets the plantuml server url to the e2e compose service via the API
// - a wiki page with a ```plantuml block renders the svg as an image
test('an enabled plantuml block in a wiki page renders as an image', async ({
    page,
    request,
    baseURL
}) => {
    await setPlantUmlUrl(request, baseURL!, 'http://e2e_plantuml:8080');
    const user = await createUser(request, baseURL!, 'plantuml-wiki');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'PlantUML Wiki');

    await Interaction.createWikiPage(page, idProject, 'Release flow', `Diagram:\n\n${BLOCK}`);
    await expect(page.getByTestId('plantuml-diagram-img')).toBeVisible();
    await expect(page.getByTestId('plantuml-diagram-img')).toHaveAttribute(
        'src',
        /^data:image\/svg\+xml/
    );
});

// Scenario:
// - with the server configured, the same block typed into a task comment
//   renders as an image in the activity feed
test('an enabled plantuml block in a task comment renders as an image', async ({
    page,
    request,
    baseURL
}) => {
    await setPlantUmlUrl(request, baseURL!, 'http://e2e_plantuml:8080');
    const user = await createUser(request, baseURL!, 'plantuml-comment');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'PlantUML Comment');

    const idIssue = await Interaction.createIssue(page, idProject, {
        title: 'PlantUML task',
        description: 'Diagram in comment.',
        state: 'New',
        severity: 'Medium'
    });
    await commentOnIssue(page, idProject, idIssue, `Diagram:\n${BLOCK}`);
    await expect(page.getByTestId('plantuml-diagram-img')).toBeVisible();
});
