import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { takeUntilDestroyed, toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute } from '@angular/router';
import { debounceTime, filter, map } from 'rxjs';
import { User } from 'src/app/auth/model/user.model';
import { ProjectMemberStore } from 'src/app/project/project-member.store';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { NoticeService } from 'src/app/shared/notice/notice.service';
import { WikiApi } from '../../api/wiki.api.service';
import { WikiSpaceKind } from '../../constants/wiki-space-kind.enum';
import { WikiProposalConverter } from '../../converter/wiki-proposal.converter';
import { WikiProposalGroups, WikiProposalTag } from '../../entity/wiki-proposal-view.entity';
import { WikiProposal } from '../../model/wiki-proposal.model';

const ALL_SPACES = 'all';

@Component({
    selector: 'app-wiki-proposals',
    templateUrl: './wiki-proposals.page.html',
    styleUrls: ['./wiki-proposals.page.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiProposalsPage {
    private readonly route = inject(ActivatedRoute);
    private readonly api = inject(WikiApi);
    private readonly i18n = inject(I18nService);

    protected readonly idProject = Number(this.route.snapshot.paramMap.get('idProject'));
    protected readonly proposals = signal<WikiProposal[] | null>(null);
    protected readonly space = signal<string>(ALL_SPACES);

    private readonly usersMap = toSignal(inject(ProjectMemberStore).usersMap$, {
        initialValue: new Map<number, User>()
    });

    private readonly idSelectedParam = toSignal(
        this.route.paramMap.pipe(map(params => Number(params.get('idProposal')) || null)),
        { initialValue: Number(this.route.snapshot.paramMap.get('idProposal')) || null }
    );

    protected readonly spaceOptions = [
        { label: this.i18n.instant('WIKI.PROPOSAL.ALL_SPACES'), value: ALL_SPACES },
        { label: this.i18n.instant('WIKI.SPACE.PROJECT'), value: WikiSpaceKind.Project },
        { label: this.i18n.instant('WIKI.SPACE.INSTANCE'), value: WikiSpaceKind.Instance }
    ];

    protected readonly groups = computed<WikiProposalGroups>(() => {
        const space = this.space();
        return WikiProposalConverter.toGroups(
            this.proposals() ?? [],
            space === ALL_SPACES ? null : (space as WikiSpaceKind)
        );
    });

    protected readonly idSelected = computed<number | null>(
        () => this.idSelectedParam() ?? this.groups().toReview[0]?.idProposal ?? null
    );

    public constructor() {
        this.load();
        inject(NoticeService)
            .wikiProposal$.pipe(
                filter(notice => notice.payload.idProject === this.idProject),
                debounceTime(300),
                takeUntilDestroyed()
            )
            .subscribe(() => this.load());
    }

    protected kindTag(proposal: WikiProposal): WikiProposalTag {
        return WikiProposalConverter.toKindTag(proposal.kind);
    }

    protected stateTag(proposal: WikiProposal): WikiProposalTag {
        return WikiProposalConverter.toStateTag(proposal.state);
    }

    protected agentName(proposal: WikiProposal): string {
        return this.usersMap().get(proposal.idUserAgent)?.name ?? '—';
    }

    protected onSpaceChange(space: string): void {
        this.space.set(space);
    }

    protected onDecided(): void {
        this.load();
    }

    private load(): void {
        this.api
            .loadOpenProposals$(this.idProject)
            .subscribe(proposals => this.proposals.set(proposals));
    }
}
