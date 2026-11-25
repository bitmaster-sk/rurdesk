const SECTION_URL = /^\/project\/\d+\/(view|wiki|release|settings)(?:[/?#]|$)/;
const ISSUE_VIEW_URL = /^\/project\/\d+\/issue\/view\/([\w-]+)(?:[/?#]|$)/;

export abstract class ProjectSwitchTarget {
    /**
     * Where switching to another project lands: the same section of the new project, never a
     * child id (issue, wiki page) that belongs to the old one. Global pages land on the overview.
     */
    public static url(currentUrl: string, idProject: number): unknown[] {
        const issueView = ISSUE_VIEW_URL.exec(currentUrl);
        if (issueView) {
            return ['/project', idProject, 'issue', 'view', issueView[1]];
        }
        const section = SECTION_URL.exec(currentUrl);
        return ['/project', idProject, section ? section[1] : 'view'];
    }
}
