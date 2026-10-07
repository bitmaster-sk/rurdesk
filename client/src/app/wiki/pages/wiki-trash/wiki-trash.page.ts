import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { User } from 'src/app/auth/model/user.model';
import { ActivatedRoute, Router } from '@angular/router';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiTrashItem } from '../../model/wiki-tree.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';

const RETENTION_DAYS = 30;
const DAY_MS = 24 * 60 * 60 * 1000;

@Component({
    selector: 'app-wiki-trash',
    templateUrl: './wiki-trash.page.html',
    styleUrls: ['./wiki-trash.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiTrashPage {
    private readonly route = inject(ActivatedRoute);
    private readonly router = inject(Router);
    private readonly api = inject(WikiApi);
    private readonly toast = inject(ToastNotificationService);
    protected readonly store = inject(WikiTreeStore);

    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly items = signal<WikiTrashItem[] | null>(null);

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    protected readonly canManageShared = computed(() => !!this.store.sharedSpace()?.canManage);
    protected readonly canManageProject = computed(() => !!this.store.projectSpace()?.canManage);

    public constructor() {
        this.load();
    }

    protected authorName(idUser: number | null): string {
        return idUser ? (this.usersMap().get(idUser)?.name ?? '—') : '—';
    }

    protected isShared(item: WikiTrashItem): boolean {
        return item.idSpace === this.store.sharedSpace()?.idSpace;
    }

    protected canPurge(item: WikiTrashItem): boolean {
        return this.isShared(item) ? this.canManageShared() : this.canManageProject();
    }

    protected daysLeft(item: WikiTrashItem): number {
        const deletedAt = new Date(
            item.deletedAt.endsWith('Z') ? item.deletedAt : `${item.deletedAt}Z`
        );
        const left = RETENTION_DAYS - Math.floor((Date.now() - deletedAt.getTime()) / DAY_MS);
        return Math.max(0, left);
    }

    protected onRestore(item: WikiTrashItem): void {
        this.api.restore$(item.idPage).subscribe(page => {
            this.toast.showSuccess('WIKI.TRASH.RESTORED');
            this.store.reload();
            void this.router.navigate([
                '/project',
                this.idProject,
                'wiki',
                this.store.kindOfSpace(page.idSpace),
                page.slug
            ]);
        });
    }

    protected onPurge(item: WikiTrashItem): void {
        this.api.purge$(item.idPage).subscribe(() => {
            this.toast.showSuccess('WIKI.TRASH.PURGED');
            this.load();
        });
    }

    private load(): void {
        this.api.loadTrash$(this.idProject).subscribe(items => this.items.set(items));
    }
}
