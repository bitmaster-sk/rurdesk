import { WikiSpaceKind } from '../constants/wiki-space-kind.enum';

export interface WikiPagePick {
    idPage: number;
    title: string;
    depth: number;
    isLinked: boolean;
}

export interface WikiPagePickGroup {
    kind: WikiSpaceKind;
    pages: WikiPagePick[];
}
