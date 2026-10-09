import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    computed,
    effect,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { Router } from '@angular/router';
import { debounceTime, filter } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { ApiError } from 'src/app/shared/model/api-error.model';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiProposalAction } from '../../constants/wiki-proposal-action.enum';
import { WikiProposalKind } from '../../constants/wiki-proposal-kind.enum';
import { WikiProposalState } from '../../constants/wiki-proposal-state.enum';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiMergeConverter } from '../../converter/wiki-merge.converter';
import { WikiProposalConverter } from '../../converter/wiki-proposal.converter';
import { WikiProposalEditLink, WikiProposalTag } from '../../entity/wiki-proposal-view.entity';
import { WikiProposal, WikiProposalDetail } from '../../model/wiki-proposal.model';
import { WikiTreeStore } from '../../store/wiki-tree.store';

@Component({
    selector: 'app-wiki-proposal-detail',
    templateUrl: './wiki-proposal-detail.component.html',
    styleUrls: ['./wiki-proposal-detail.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiProposalDetailComponent {
    private readonly api = inject(WikiApi);
    private readonly router = inject(Router);
    private readonly toast = inject(ToastNotificationService);
    private readonly store = inject(WikiTreeStore);
    private readonly destroyRef = inject(DestroyRef);
    private readonly i18n = inject(I18nService);

    public readonly idProject = input.required<number>();
    public readonly idProposal = input.required<number>();

    public readonly decided = output<WikiProposal>();

    protected readonly Kind = WikiProposalKind;
    protected readonly State = WikiProposalState;
    protected readonly Action = WikiProposalAction;

    protected readonly detail = signal<WikiProposalDetail | null>(null);
    protected readonly isAccepting = signal(false);
    protected readonly isRejecting = signal(false);
    protected readonly isRejectOpen = signal(false);
    protected readonly rejectReason = signal('');

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    protected readonly proposal = computed(() => this.detail()?.proposal ?? null);

    protected readonly kindTag = computed<WikiProposalTag | null>(() => {
        const proposal = this.proposal();
        return proposal ? WikiProposalConverter.toKindTag(proposal.kind) : null;
    });

    protected readonly stateTag = computed<WikiProposalTag | null>(() => {
        const proposal = this.proposal();
        return proposal ? WikiProposalConverter.toStateTag(proposal.state) : null;
    });

    protected readonly action = computed<WikiProposalAction | null>(() => {
        const proposal = this.proposal();
        return proposal ? WikiProposalConverter.toAction(proposal.state) : null;
    });

    protected readonly isApproved = computed(
        () => this.proposal()?.state === WikiProposalState.Approved
    );

    protected readonly isNeedsResolving = computed(
        () => this.proposal()?.state === WikiProposalState.NeedsResolving
    );

    protected readonly editLabel = computed(() => {
        if (this.isNeedsResolving()) {
            return 'WIKI.PROPOSAL.RESOLVE';
        }
        if (this.isApproved()) {
            return 'WIKI.PROPOSAL.EDIT';
        }
        return this.action() === WikiProposalAction.Approve
            ? 'WIKI.PROPOSAL.EDIT_APPROVE'
            : 'WIKI.PROPOSAL.EDIT_PUBLISH';
    });

    protected readonly canDecide = computed(() => {
        const proposal = this.proposal();
        if (!proposal) {
            return false;
        }
        const space =
            proposal.spaceKind === WikiSpaceKind.Instance
                ? this.store.sharedSpace()
                : this.store.projectSpace();
        return !!space?.canEdit;
    });

    protected readonly editLink = computed<WikiProposalEditLink | null>(() => {
        const proposal = this.proposal();
        return proposal ? WikiProposalConverter.toEditLink(proposal, this.idProject()) : null;
    });

    protected readonly pageLink = computed(() => {
        const proposal = this.proposal();
        return proposal ? WikiProposalConverter.toPageLink(proposal, this.idProject()) : null;
    });

    protected readonly isPageChanged = computed(() => {
        const proposal = this.proposal();
        return (
            proposal?.kind === WikiProposalKind.Update &&
            proposal.pageVersion !== null &&
            proposal.baseVersion !== null &&
            proposal.pageVersion > proposal.baseVersion
        );
    });

    protected readonly moveParams = computed(() => {
        const proposal = this.proposal();
        const root = this.i18n.instant('WIKI.MOVE.ROOT');
        return {
            page: proposal?.pageTitle ?? proposal?.title ?? '',
            from: proposal?.pageParentTitle ?? root,
            to: proposal?.parentTitle ?? proposal?.parentSlug ?? root
        };
    });

    protected readonly agentName = computed(() => {
        const proposal = this.proposal();
        return proposal ? (this.usersMap().get(proposal.idUserAgent)?.name ?? '—') : '';
    });

    protected readonly deciderName = computed(() => {
        const idUser = this.proposal()?.decidedBy;
        return idUser ? (this.usersMap().get(idUser)?.name ?? '—') : '—';
    });

    public constructor() {
        effect(() => {
            const idProposal = this.idProposal();
            this.detail.set(null);
            this.load(idProposal);
        });
        inject(NoticeService)
            .wikiProposal$.pipe(
                filter(notice => notice.payload.idIssue === this.proposal()?.idIssue),
                debounceTime(300),
                takeUntilDestroyed()
            )
            .subscribe(() => this.load(this.idProposal()));
    }

    protected onAccept(): void {
        const proposal = this.proposal();
        if (!proposal || this.isAccepting()) {
            return;
        }
        this.isAccepting.set(true);
        this.api.acceptProposal$(proposal.idProposal, {}).subscribe({
            next: result => {
                this.isAccepting.set(false);
                this.toast.showSuccess(
                    result.proposal.state === WikiProposalState.Approved
                        ? 'WIKI.PROPOSAL.APPROVED'
                        : 'WIKI.PROPOSAL.ACCEPTED'
                );
                this.store.reload();
                this.applyDecision(result.proposal);
            },
            error: (error: unknown) => {
                this.isAccepting.set(false);
                const link = this.editLink();
                if (WikiMergeConverter.toConflict(error) && link) {
                    this.toast.showInfo('WIKI.PROPOSAL.CONFLICT');
                    void this.router.navigate(link.commands, { queryParams: link.queryParams });
                    return;
                }
                this.toast.showError(ApiError.translateKeyOf(error) ?? 'error.internal');
                this.load(proposal.idProposal);
            }
        });
    }

    protected onEdit(link: WikiProposalEditLink): void {
        void this.router.navigate(link.commands, { queryParams: link.queryParams });
    }

    protected onReasonInput(event: Event): void {
        if (event.target instanceof HTMLTextAreaElement) {
            this.rejectReason.set(event.target.value);
        }
    }

    protected onReject(): void {
        const proposal = this.proposal();
        if (!proposal || this.isRejecting()) {
            return;
        }
        this.isRejecting.set(true);
        this.api.rejectProposal$(proposal.idProposal, this.rejectReason().trim()).subscribe({
            next: rejected => {
                this.isRejecting.set(false);
                this.isRejectOpen.set(false);
                this.rejectReason.set('');
                this.toast.showSuccess('WIKI.PROPOSAL.REJECTED');
                this.store.reload();
                this.applyDecision(rejected);
            },
            error: () => this.isRejecting.set(false)
        });
    }

    private applyDecision(proposal: WikiProposal): void {
        this.detail.update(detail => (detail ? { ...detail, proposal } : detail));
        this.decided.emit(proposal);
    }

    private load(idProposal: number): void {
        this.api
            .loadProposal$(idProposal)
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe(detail => {
                if (detail.proposal.idProposal === this.idProposal()) {
                    this.detail.set(detail);
                }
            });
    }
}
