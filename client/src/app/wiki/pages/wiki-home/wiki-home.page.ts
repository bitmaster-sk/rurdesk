import { ChangeDetectionStrategy, Component, computed, effect, inject } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-home',
    templateUrl: './wiki-home.page.html',
    styleUrls: ['./wiki-home.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiHomePage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    protected readonly store = inject(WikiTreeStore);

    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));

    protected readonly pageCount = computed(() => this.store.tree()?.nodes.length ?? 0);
    protected readonly canCreate = computed(() => !!this.store.projectSpace()?.canEdit);

    private readonly homeNode = computed(() => {
        const idHomePage = this.store.projectSpace()?.idHomePage;
        return this.store.tree()?.nodes.find(node => node.idPage === idHomePage) ?? null;
    });

    protected readonly isIntroShown = computed(() => !!this.store.tree() && !this.homeNode());

    public constructor() {
        effect(() => {
            const home = this.homeNode();
            if (home) {
                void this.router.navigate(
                    ['/project', this.idProject, 'wiki', WikiSpaceKind.Project, home.slug],
                    { replaceUrl: true }
                );
            }
        });
    }

    protected onCreate(): void {
        void this.router.navigate(['/project', this.idProject, 'wiki', 'new'], {
            queryParams: { space: WikiSpaceKind.Project }
        });
    }
}
