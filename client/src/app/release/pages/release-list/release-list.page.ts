import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    inject,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { merge } from 'rxjs';
import { debounceTime } from 'rxjs/operators';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { AclStore } from 'src/app/project/store/acl.store';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseView } from '../../model/release.model';

@Component({
    selector: 'app-release-list',
    templateUrl: './release-list.page.html',
    host: { class: 'block h-full overflow-y-auto' },
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class ReleaseListPage {
    private readonly route = inject(ActivatedRoute);
    private readonly api = inject(ReleaseApi);
    private readonly destroyRef = inject(DestroyRef);
    protected readonly acl = inject(AclStore);

    protected readonly ReleaseStatus = ReleaseStatus;
    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly releases = signal<ReleaseView[] | null>(null);

    private readonly users = toSignal(inject(ProjectMemberStore).users$, { initialValue: [] });
    private readonly usersMap = computed(
        () => new Map(this.users().map(user => [user.idUser, user]))
    );

    public constructor() {
        this.load();
        const notices = inject(NoticeService);
        merge(notices.release$, notices.agentRun$)
            .pipe(debounceTime(300), takeUntilDestroyed(this.destroyRef))
            .subscribe(() => this.load());
    }

    protected authorName(release: ReleaseView): string {
        const idUser = release.releasedBy ?? release.createBy;
        return idUser ? (this.usersMap().get(idUser)?.name ?? '—') : '—';
    }

    private load(): void {
        this.api.load$(this.idProject).subscribe(releases => this.releases.set(releases));
    }
}
