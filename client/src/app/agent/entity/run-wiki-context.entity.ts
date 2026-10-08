export interface RunWikiPage {
    key: number;
    title: string;
    versionNo: number | null;
    tokens: number;
    link: (string | number)[] | null;
}

export interface RunWikiPromptPage extends RunWikiPage {
    isAlways: boolean;
}

export interface RunWikiCall {
    idCall: string;
    isSearch: boolean;
    target: string;
    pages: RunWikiPage[];
}

export interface RunWikiStage {
    stage: string;
    prompt: RunWikiPromptPage[];
    index: RunWikiPage[];
    calls: RunWikiCall[];
}
