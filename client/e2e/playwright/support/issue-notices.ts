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

interface CustomFieldNotice {
    subject: string;
    action: string;
    payload: {
        key?: string;
        customFields?: Record<string, string | number | boolean | null>;
    };
}

/** Collects the `custom_field` and `issue` websocket notices the page receives. */
export class CustomFieldNoticeLog {
    private readonly notices: CustomFieldNotice[] = [];

    private readonly subject: string;

    private constructor(subject: string) {
        this.subject = subject;
    }

    public static attach(page: Page): CustomFieldNoticeLog {
        return CustomFieldNoticeLog.collect(page, 'custom_field');
    }

    public static attachIssueNotices(page: Page): CustomFieldNoticeLog {
        return CustomFieldNoticeLog.collect(page, 'issue');
    }

    private static collect(page: Page, subject: string): CustomFieldNoticeLog {
        const log = new CustomFieldNoticeLog(subject);
        page.on('websocket', socket => {
            socket.on('framereceived', frame => {
                let parsed: CustomFieldNotice;
                try {
                    parsed = JSON.parse(frame.payload as string) as CustomFieldNotice;
                } catch {
                    return;
                }
                if (parsed?.subject === subject) {
                    log.notices.push(parsed);
                }
            });
        });
        return log;
    }

    public async waitForAction(action: string, timeoutMs: number): Promise<void> {
        await expect
            .poll(() => this.notices.some(notice => notice.action === action), {
                timeout: timeoutMs,
                message: `no ${this.subject} notice with action ${action}`
            })
            .toBe(true);
    }

    public async waitForCustomFieldValue(
        key: string,
        value: string,
        timeoutMs: number
    ): Promise<void> {
        await expect
            .poll(
                () => this.notices.some(notice => notice.payload?.customFields?.[key] === value),
                {
                    timeout: timeoutMs,
                    message: `no ${this.subject} notice carried ${key}=${value}`
                }
            )
            .toBe(true);
    }
}
