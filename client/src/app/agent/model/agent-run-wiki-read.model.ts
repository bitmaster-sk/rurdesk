import { WikiSpaceKind } from 'src/app/wiki/constants/wiki-space-kind.enum';
import { AgentWikiReadSource } from './agent-wiki-read-source.enum';

export interface AgentRunWikiRead {
    idRead: number;
    idRun: number;
    idTask: number | null;
    stage: string;
    idCall: string;
    source: AgentWikiReadSource;
    idPage: number | null;
    spaceKind: WikiSpaceKind | null;
    slug: string | null;
    title: string | null;
    versionNo: number | null;
    tokens: number;
    query: string | null;
    pages: AgentRunWikiIndexPage[] | null;
    createAt: string;
}

export interface AgentRunWikiIndexPage {
    idPage: number;
    spaceKind: WikiSpaceKind;
    slug: string;
    title: string;
}
