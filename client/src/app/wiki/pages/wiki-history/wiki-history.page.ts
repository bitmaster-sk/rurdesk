import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { User } from 'src/app/auth/model/user.model';
import { ActivatedRoute, Router } from '@angular/router';
import { switchMap } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiPageView } from '../../model/wiki-page.model';
import { WikiVersion, WikiVersionSummary } from '../../model/wiki-version.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-history',
    templateUrl: './wiki-history.page.html',
    styleUrls: ['./wiki-history.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiHistoryPage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(WikiApi);
    private readonly toast = inject(ToastNotificationService);
    private readonly treeStore = inject(WikiTreeStore);

    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly view = signal<WikiPageView | null>(null);
    protected readonly versions = signal<WikiVersionSummary[]>([]);
    protected readonly selected = signal<number | null>(null);
    protected readonly diff = signal<string | null>(null);
    protected readonly firstVersion = signal<WikiVersion | null>(null);

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    protected readonly latest = computed(() => this.versions()[0]?.versionNo ?? null);

    public constructor() {
        this.route.paramMap
            .pipe(
                switchMap(params =>
                    this.api.loadOne$(
                        this.idProject,
                        params.get('space') as WikiSpaceKind,
                        params.get('slug') ?? ''
                    )
                ),
                takeUntilDestroyed()
            )
            .subscribe(view => {
                this.view.set(view);
                this.loadVersions(view.page.idPage);
            });
    }

    protected authorName(idUser: number | null): string {
        return idUser ? (this.usersMap().get(idUser)?.name ?? '—') : '—';
    }

    protected onSelect(versionNo: number): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.selected.set(versionNo);
        this.diff.set(null);
        this.firstVersion.set(null);
        if (versionNo === 1) {
            this.api
                .loadVersion$(view.page.idPage, 1)
                .subscribe(version => this.firstVersion.set(version));
            return;
        }
        this.api
            .loadDiff$(view.page.idPage, versionNo - 1, versionNo)
            .subscribe(result => this.diff.set(result.diff));
    }

    protected onRevert(versionNo: number): void {
        const view = this.view();
        if (!view) {
            return;
        }
        this.api.revert$(view.page.idPage, versionNo).subscribe(page => {
            this.toast.showSuccess('WIKI.HISTORY.RESTORED');
            this.treeStore.reload();
            void this.router.navigate([
                '/project',
                this.idProject,
                'wiki',
                view.spaceKind,
                page.slug
            ]);
        });
    }

    private loadVersions(idPage: number): void {
        this.api.loadVersions$(idPage).subscribe(versions => {
            this.versions.set(versions);
            if (versions.length > 0) {
                this.onSelect(versions[0].versionNo);
            }
        });
    }
}
