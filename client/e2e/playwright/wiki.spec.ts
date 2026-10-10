import { APIRequestContext, expect, test } from '@playwright/test';
import { Interaction } from './support/interaction';
import { createUser } from './support/user';

interface WikiPageResponse {
    page: { idPage: number; versionNo: number; body: string };
}

async function loadWikiPage(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idProject: number,
    slug: string
): Promise<WikiPageResponse> {
    const res = await request.get(
        `${baseURL}/api/private/project/${idProject}/wiki/page/project/${slug}`,
        {
            headers: { Authorization: token }
        }
    );
    expect(res.status(), await res.text()).toBe(200);
    return (await res.json()) as WikiPageResponse;
}

async function saveWikiPageViaApi(
    request: APIRequestContext,
    baseURL: string,
    token: string,
    idPage: number,
    baseVersion: number,
    title: string,
    body: string
): Promise<void> {
    const res = await request.put(`${baseURL}/api/private/wiki/page/${idPage}`, {
        headers: { Authorization: token },
        data: { baseVersion, title, body, summary: '', agentAccess: 'on_demand', note: 'from api' }
    });
    expect(res.status(), await res.text()).toBe(200);
}

// Scenario:
// - create a dedicated user, log in and create a blank project
// - open the wiki and assert the main menu collapsed to icons
// - create a page with a link to a page that does not exist yet
// - edit the page and save a second version
// - open the history, restore v1 and assert the page shows the v1 text again
test('a wiki page is created, edited and restored from history', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-history');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki History Project');

    await page.getByTestId('sidebar-link-wiki').click();
    await expect(page.getByTestId('wiki-home')).toBeVisible();
    await expect(page.getByTestId('project-left-menu')).toHaveClass(/home-left-menu--collapsed/);

    await Interaction.createWikiPage(
        page,
        idProject,
        'Konvencie backendu',
        'Controller only parses the request.\n\nSee [[Prehľad systému]].'
    );
    await expect(page.getByTestId('wiki-page-body')).toContainText(
        'Controller only parses the request.'
    );
    await expect(
        page.getByTestId('wiki-page-body').getByRole('link', { name: 'Prehľad systému' })
    ).toHaveAttribute('href', /\/wiki\/new\?space=project&title=/);
    await expect(page.getByTestId('wiki-tree-project').getByTestId('wiki-tree-node')).toHaveText([
        'Konvencie backendu'
    ]);

    await page.getByTestId('wiki-edit').click();
    await Interaction.fillWikiEditor(page, 'Controller calls the service layer.');
    await page.getByTestId('wiki-save').click();
    await expect(page.getByTestId('wiki-page-body')).toContainText(
        'Controller calls the service layer.'
    );

    await page.getByTestId('wiki-history-link').click();
    await expect(page.getByTestId('wiki-version')).toHaveCount(2);
    await page.getByTestId('wiki-version').nth(1).click();
    await page.getByTestId('wiki-revert').click();
    await Interaction.acceptConfirm(page);

    await expect(page.getByTestId('wiki-page-body')).toContainText(
        'Controller only parses the request.'
    );
    await page.getByTestId('wiki-history-link').click();
    await expect(page.getByTestId('wiki-version')).toHaveCount(3);
});

// Scenario:
// - create a page and open it in the editor (based on v1)
// - meanwhile save v2 through the API, changing only the first line
// - append a line in the editor and save
// - assert the save succeeds and the page holds both changes
// - open the editor again, rename the page and change the same line through the API, then change that line in the editor too
// - assert the conflict screen appears, keep "mine" and assert the saved page shows it under the new title
test('concurrent wiki edits merge, and overlapping ones ask which side to keep', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-merge');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Merge Project');
    const token = await Interaction.apiToken(request, baseURL!, user);

    await Interaction.createWikiPage(page, idProject, 'Merge Rules', 'first line\nsecond line');
    const created = await loadWikiPage(request, baseURL!, token, idProject, 'merge-rules');

    await page.getByTestId('wiki-edit').click();
    await expect(page.getByTestId('wiki-editor')).toBeVisible();
    await saveWikiPageViaApi(
        request,
        baseURL!,
        token,
        created.page.idPage,
        1,
        'Merge Rules',
        'first line edited\nsecond line'
    );
    await Interaction.fillWikiEditor(page, 'first line\nsecond line\nthird line');
    await page.getByTestId('wiki-save').click();
    await expect(page.getByTestId('wiki-page-body')).toContainText('first line edited');
    await expect(page.getByTestId('wiki-page-body')).toContainText('third line');

    const merged = await loadWikiPage(request, baseURL!, token, idProject, 'merge-rules');
    expect(merged.page.versionNo).toBe(3);

    await page.getByTestId('wiki-edit').click();
    await expect(page.getByTestId('wiki-editor')).toBeVisible();
    await saveWikiPageViaApi(
        request,
        baseURL!,
        token,
        merged.page.idPage,
        3,
        'Merge Rules Renamed',
        'first line theirs\nsecond line\nthird line'
    );
    await Interaction.fillWikiEditor(page, 'first line mine\nsecond line\nthird line');
    await page.getByTestId('wiki-save').click();

    await expect(page.getByTestId('wiki-conflict-chunk')).toHaveCount(1);
    await page.getByTestId('wiki-conflict-mine').click();
    await page.getByTestId('wiki-conflict-save').click();
    await expect(page.getByTestId('wiki-page-body')).toContainText('first line mine');
    await expect(page.getByTestId('wiki-page-title')).toHaveText('Merge Rules Renamed');
});

// Scenario:
// - create a parent page and a child page under it
// - move the parent to the trash and assert both disappear from the tree
// - assert the trash link counts one item, since subpages go with their parent
// - restore the parent from the trash and assert both are back and the count is gone
test('a page moved to the trash comes back with its subpages', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-trash');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Trash Project');
    const token = await Interaction.apiToken(request, baseURL!, user);

    await Interaction.createWikiPage(page, idProject, 'Operations', 'parent');
    const parent = await loadWikiPage(request, baseURL!, token, idProject, 'operations');
    const child = await request.post(`${baseURL}/api/private/project/${idProject}/wiki/page`, {
        headers: { Authorization: token },
        data: { space: 'project', idParent: parent.page.idPage, title: 'Deployment', body: 'child' }
    });
    expect(child.status(), await child.text()).toBe(200);

    await page.goto(`/project/${idProject}/wiki/project/operations`);
    await page.getByTestId('wiki-delete').click();
    await Interaction.acceptConfirm(page);
    await expect(page.getByTestId('wiki-home')).toBeVisible();
    await expect(page.getByTestId('wiki-tree-project').getByTestId('wiki-tree-node')).toHaveCount(
        0
    );
    await expect(page.getByTestId('wiki-trash-count')).toHaveText('1');

    await page.getByTestId('wiki-trash-link').click();
    await expect(page.getByTestId('wiki-trash-row')).toHaveCount(1);
    await expect(page.getByTestId('wiki-trash-row')).toContainText('Operations');
    await page.getByTestId('wiki-trash-restore').click();

    await expect(page.getByTestId('wiki-page-title')).toHaveText('Operations');
    await expect(page.getByTestId('wiki-tree-project').getByTestId('wiki-tree-node')).toHaveCount(
        1
    );
    await expect(page.getByTestId('wiki-trash-count')).toHaveCount(0);
    await page.getByTestId('wiki-tree-project').locator('.wiki-tree__chevron').click();
    await expect(page.getByTestId('wiki-tree-project').getByTestId('wiki-tree-node')).toHaveText([
        /Operations/,
        /Deployment/
    ]);
});

// Scenario:
// - create a page with a long paragraph on a wide screen
// - switch the page to full width and assert the text gets wider
// - reload and assert the full width choice is kept
test('a wiki page can switch to full width and keeps the choice', async ({
    page,
    request,
    baseURL
}) => {
    await page.setViewportSize({ width: 1700, height: 800 });
    const user = await createUser(request, baseURL!, 'wiki-width');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki width');
    await Interaction.createWikiPage(page, idProject, 'Long page', 'word '.repeat(300));
    const body = page.getByTestId('wiki-page-body');
    const width = async (): Promise<number> => (await body.boundingBox())?.width ?? 0;
    const narrow = await width();

    await page.getByTestId('wiki-full-width').click();
    await expect.poll(width).toBeGreaterThan(narrow + 100);

    await page.reload();
    await expect(page.getByTestId('wiki-page-title')).toHaveText('Long page');
    await expect.poll(width).toBeGreaterThan(narrow + 100);
});

// Scenario:
// - create a page, hide the page tree and assert only the show button is left in the content
// - reload and assert the tree stays hidden
// - show the tree again and assert the page is listed in it
test('the wiki page tree can be hidden and keeps that after a reload', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-tree-toggle');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki tree toggle');
    await Interaction.createWikiPage(page, idProject, 'Tree page', 'text');

    await page.getByTestId('wiki-tree-hide').click();
    await expect(page.getByTestId('wiki-nav')).toHaveCount(0);
    await expect(page.getByTestId('wiki-tree-show')).toBeVisible();

    await page.reload();
    await expect(page.getByTestId('wiki-page-title')).toHaveText('Tree page');
    await expect(page.getByTestId('wiki-nav')).toHaveCount(0);

    await page.getByTestId('wiki-tree-show').click();
    await expect(page.getByTestId('wiki-tree-project')).toContainText('Tree page');
});

// Scenario:
// - start a new page, type a title and text, then click the trash link in the wiki menu
// - assert the "discard this new page?" dialog appears, keep writing and assert the text is still there
// - click the trash link again, discard, and assert the trash page opens
// - edit an existing page, type a line and leave right away through the trash link
// - open the editor again and assert the server kept a draft with that line
test('leaving the editor never drops what was written without asking', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-leave');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Leave Project');

    await page.goto(`/project/${idProject}/wiki/new?space=project`);
    await page.getByTestId('wiki-edit-title').fill('Unsaved idea');
    await Interaction.fillWikiEditor(page, 'a long thought');
    await page.getByTestId('wiki-trash-link').click();
    await expect(page.getByTestId('wiki-leave-stay')).toBeVisible();
    await page.getByTestId('wiki-leave-stay').click();
    await expect(page.getByTestId('wiki-edit-title')).toHaveValue('Unsaved idea');
    await expect(page.getByTestId('wiki-editor')).toContainText('a long thought');

    await page.getByTestId('wiki-trash-link').click();
    await page.getByTestId('wiki-leave-discard').click();
    await expect(page).toHaveURL(/\/wiki\/trash$/);

    await Interaction.createWikiPage(page, idProject, 'Kept Draft', 'first');
    await page.getByTestId('wiki-edit').click();
    await Interaction.fillWikiEditor(page, 'first\nwritten just before leaving');
    await page.getByTestId('wiki-trash-link').click();
    await expect(page).toHaveURL(/\/wiki\/trash$/);

    await page.goto(`/project/${idProject}/wiki/project/kept-draft/edit`);
    await expect(page.getByTestId('wiki-draft-continue')).toBeVisible();
    await page.getByTestId('wiki-draft-continue').click();
    await expect(page.getByTestId('wiki-editor')).toContainText('written just before leaving');
});

// Scenario:
// - create a long page with a link to its own heading near the bottom
// - click that link and assert the URL gets the heading anchor and the heading scrolls into view
// - create a second page that links to the heading of the first page
// - click it and assert the first page opens scrolled to the heading
test('links can point at a heading on the same or another page', async ({
    page,
    request,
    baseURL
}) => {
    await page.setViewportSize({ width: 1400, height: 700 });
    const user = await createUser(request, baseURL!, 'wiki-anchor');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Anchor Project');
    const filler = Array.from({ length: 80 }, (_, index) => `line ${index}`).join('\n\n');
    await Interaction.createWikiPage(
        page,
        idProject,
        'Runbook',
        `[[#Rollback plán|jump to rollback]]\n\n${filler}\n\n## Rollback plán\n\nrollback steps`
    );
    const heading = page.locator('#wiki-h-rollback-plan');

    await page.getByRole('link', { name: 'jump to rollback' }).click();
    await expect(page).toHaveURL(/\/wiki\/project\/runbook#rollback-plan$/);
    await expect(heading).toBeInViewport();

    await Interaction.createWikiPage(page, idProject, 'Index', 'see [[Runbook#Rollback plán]]');
    await page.getByRole('link', { name: 'Runbook › Rollback plán' }).click();
    await expect(page).toHaveURL(/\/wiki\/project\/runbook#rollback-plan$/);
    await expect(heading).toBeInViewport();
});

// Scenario:
// - create a page and mark it as the wiki home page
// - open the wiki from the project menu and assert it lands on that page
// - stop using it as the home page and assert the wiki opens the start screen again
test('the owner can pick the page the wiki opens with', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'wiki-home');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Home Project');
    await Interaction.createWikiPage(page, idProject, 'Start here', 'welcome');

    await page.getByTestId('wiki-home-toggle').click();
    await expect(page.getByTestId('wiki-home-toggle')).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByTestId('wiki-home-toggle')).toHaveText('Unset as home');
    await page.getByTestId('sidebar-link-wiki').click();
    await expect(page).toHaveURL(/\/wiki\/project\/start-here$/);
    await expect(page.getByTestId('wiki-page-title')).toHaveText('Start here');

    await page.getByTestId('wiki-home-toggle').click();
    await expect(page.getByTestId('wiki-home-toggle')).toHaveAttribute('aria-pressed', 'false');
    await expect(page.getByTestId('wiki-home-toggle')).toHaveText('Set as home');
    await page.getByTestId('sidebar-link-wiki').click();
    await expect(page.getByTestId('wiki-home')).toBeVisible();
});

// Scenario:
// - create a wiki page and a task in a new project
// - on the task detail open the wiki picker and assert it lists the page tree
// - type part of the page title, pick the page and assert the task lists it as linked
test('a wiki page can be linked to a task from the task detail', async ({
    page,
    request,
    baseURL
}) => {
    const user = await createUser(request, baseURL!, 'wiki-task-link');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Task Link Project');
    await Interaction.createWikiPage(page, idProject, 'Deploy runbook', 'steps');
    const idIssue = await Interaction.createIssue(page, idProject, {
        title: 'Ship the release',
        description: 'No links yet.',
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    await page.getByTestId('issue-wiki-add').click();
    await expect(
        page.getByTestId('issue-wiki-pick').filter({ hasText: 'Deploy runbook' })
    ).toBeVisible();
    await page.getByTestId('issue-wiki-search').fill('deplo runb');
    await expect(page.getByTestId('issue-wiki-pick')).toHaveText(['Deploy runbook']);
    await page.getByTestId('issue-wiki-pick').click();

    await expect(page.getByTestId('issue-wiki-link')).toContainText('Deploy runbook');
});

// Scenario:
// - create a wiki page and a task in a new project
// - in the task comment type [[ and part of the title, pick the page with Enter and send
// - assert the comment shows a link that opens the wiki page
test('a comment links a wiki page picked after typing [[', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'wiki-comment-link');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Comment Link Project');
    await Interaction.createWikiPage(page, idProject, 'Deploy runbook', 'steps');
    const idIssue = await Interaction.createIssue(page, idProject, {
        title: 'Ship the release',
        description: 'No links yet.',
        state: 'New',
        severity: 'Medium'
    });

    await page.goto(`/project/${idProject}/issue/${idIssue}`);
    const comment = page.locator('.feed-editor [contenteditable="true"]');
    await comment.click();
    await comment.pressSequentially('read [[deplo');
    await expect(page.getByTestId('wiki-link-option')).toHaveText(['Deploy runbook']);
    await comment.press('Enter');
    await expect(comment).toHaveText('read [[Deploy runbook]]');
    await comment.press('ControlOrMeta+Enter');

    const link = page.locator('.feed-container').getByRole('link', { name: 'Deploy runbook' });
    await link.click();
    await expect(page).toHaveURL(/\/wiki\/project\/deploy-runbook$/);
    await expect(page.getByTestId('wiki-page-title')).toHaveText('Deploy runbook');
});

// Scenario:
// - create a page whose body holds a mermaid flowchart fence
// - open the page and assert the fenced block renders as an SVG diagram,
//   not as a code block
test('a mermaid block in a wiki page renders as a diagram', async ({ page, request, baseURL }) => {
    const user = await createUser(request, baseURL!, 'wiki-mermaid');
    await Interaction.login(page, user);
    const idProject = await Interaction.createBlankProject(page, 'Wiki Mermaid Project');

    await Interaction.createWikiPage(
        page,
        idProject,
        'Release flow',
        'How a change reaches production:\n\n' +
            '```mermaid\n' +
            'flowchart TD\n' +
            '    dev --> review --> deploy\n' +
            '```\n'
    );

    const diagram = page.getByTestId('mermaid-diagram');
    await expect(diagram).toBeVisible();
    await expect(diagram.locator('svg')).toBeVisible();
});
