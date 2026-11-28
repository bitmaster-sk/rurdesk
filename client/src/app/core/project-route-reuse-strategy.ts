import { ActivatedRouteSnapshot, BaseRouteReuseStrategy } from '@angular/router';

/**
 * Builds the project pages again when the user switches to another project. Angular would reuse
 * them and only swap the `idProject` param, but most project pages read the project once on init
 * and would keep showing the previous one.
 */
export class ProjectRouteReuseStrategy extends BaseRouteReuseStrategy {
    public override shouldReuseRoute(
        future: ActivatedRouteSnapshot,
        curr: ActivatedRouteSnapshot
    ): boolean {
        return (
            super.shouldReuseRoute(future, curr) &&
            future.paramMap.get('idProject') === curr.paramMap.get('idProject')
        );
    }
}
