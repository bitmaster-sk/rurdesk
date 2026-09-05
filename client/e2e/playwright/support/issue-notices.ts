import { expect, Page } from '@playwright/test';

interface IssueNotice {
    subject: string;
    action: string;
    payload: { idIssue: number; idState: number | null };
}

/** Collects the `issue` websocket notices the page receives. */
export class IssueNoticeLog {
    private readonly notices: IssueNotice[] = [];

    private constructor() {}

    public static attach(page: Page): IssueNoticeLog {
        const log = new IssueNoticeLog();
        page.on('websocket', socket => {
            socket.on('framereceived', frame => {
                let parsed: IssueNotice;
                try {
                    parsed = JSON.parse(frame.payload as string) as IssueNotice;
                } catch {
                    return;
                }
                if (parsed?.subject === 'issue') {
                    log.notices.push(parsed);
                }
            });
        });
        return log;
    }

    public async waitForState(idState: number, timeoutMs: number): Promise<void> {
        await expect
            .poll(() => this.notices.some(notice => notice.payload?.idState === idState), {
                timeout: timeoutMs,
                message: `no issue notice carried state ${idState}`
            })
            .toBe(true);
    }
}
