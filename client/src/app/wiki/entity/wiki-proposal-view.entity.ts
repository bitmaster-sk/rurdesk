import { Params } from '@angular/router';
import { UiTagSeverity } from 'src/app/ui/components/tag/tag.component';
import { WikiProposal } from '../model/wiki-proposal.model';

export interface WikiProposalTag {
    labelKey: string;
    severity: UiTagSeverity;
}

export interface WikiProposalGroups {
    toReview: WikiProposal[];
    approved: WikiProposal[];
}

export interface WikiProposalBanner {
    key: string;
    params: Record<string, string | number | null>;
}

export interface WikiProposalEditLink {
    commands: (string | number)[];
    queryParams: Params;
}
