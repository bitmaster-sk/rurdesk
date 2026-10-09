import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { User } from 'src/app/auth/model/user.model';
import { WikiProposalConverter } from 'src/app/wiki/converter/wiki-proposal.converter';
import { WikiProposalTag } from 'src/app/wiki/entity/wiki-proposal-view.entity';
import { WikiProposal } from 'src/app/wiki/model/wiki-proposal.model';

@Component({
    selector: 'app-wiki-proposal-card',
    templateUrl: './wiki-proposal-card.component.html',
    styleUrls: ['./wiki-proposal-card.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiProposalCardComponent {
    public readonly proposal = input.required<WikiProposal>();
    public readonly agent = input<User | undefined>(undefined);

    protected readonly kindTag = computed<WikiProposalTag>(() =>
        WikiProposalConverter.toKindTag(this.proposal().kind)
    );

    protected readonly stateTag = computed<WikiProposalTag>(() =>
        WikiProposalConverter.toStateTag(this.proposal().state)
    );

    protected readonly proposalLink = computed(() =>
        WikiProposalConverter.toProposalLink(this.proposal())
    );

    protected readonly pageLink = computed(() =>
        WikiProposalConverter.toPageLink(this.proposal(), this.proposal().idProject)
    );
}
