import { CanMatchFn, UrlSegment } from '@angular/router';

export abstract class ProjectLayoutGuard {
    /**
     * Matches the ProjectLayout shell only for URLs that actually carry path
     * segments (e.g. /project/...). The ProjectLayout route uses an empty path so it
     * can wrap the /project children in its shell; without this guard that empty
     * path also greedily matches the bare '/', shadowing the sibling empty-path
     * route that hosts the first-project onboarding and the default "My page".
     */
    public static readonly canMatch: CanMatchFn = (_route, segments: UrlSegment[]) =>
        segments.length > 0;
}
