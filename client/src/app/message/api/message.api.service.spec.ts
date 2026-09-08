import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { MessageApi } from './message.api.service';
import { MessageRecipientType } from '../constant/message-recipient-type.enum';

function buildApi(http: HttpClient): MessageApi {
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: http }] });
    return runInInjectionContext(injector, () => new MessageApi());
}

describe('MessageApi.insert$', () => {
    it('includes anchor fields in the body when an anchor is given', () => {
        const post = vi.fn().mockReturnValue(of({ createdAt: '2026-01-01T00:00:00Z' }));
        const api = buildApi({ post } as unknown as HttpClient);

        api.insert$(5, MessageRecipientType.user, 'hi', {
            idParentMessage: 1,
            anchorLineStart: 2,
            anchorLineEnd: 3
        }).subscribe();

        const [url, body] = post.mock.calls[0] as [string, Record<string, unknown>];
        expect(url).toBe('/api/private/message');
        expect(body).toMatchObject({
            idRecipient: 5,
            message: 'hi',
            idParentMessage: 1,
            anchorLineStart: 2,
            anchorLineEnd: 3
        });
    });
});
