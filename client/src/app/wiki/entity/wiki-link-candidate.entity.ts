import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';

export interface WikiLinkCandidate {
    idPage: number;
    title: string;
    slug: string;
    kind: WikiSpaceKind;
}
