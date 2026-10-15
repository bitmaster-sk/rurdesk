import { ChangeDetectionStrategy, Component, computed, input } from '@angular/core';
import { WikiProposalConverter } from '../../converter/wiki-proposal.converter';
import { WikiProposal } from '../../model/wiki-proposal.model';

@Component({
    selector: 'app-wiki-proposal-origin',
    templateUrl: './wiki-proposal-origin.component.html',
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class WikiProposalOriginComponent {
    public readonly proposal = input.required<WikiProposal>();
    public readonly linkIssue = input(false);

    protected readonly authorName = computed(() =>
        WikiProposalConverter.toAuthorName(this.proposal())
    );

    protected readonly issueLink = computed(() => [
        '/project',
        this.proposal().idProject,
        'issue',
        this.proposal().idIssuePublic
    ]);
}
