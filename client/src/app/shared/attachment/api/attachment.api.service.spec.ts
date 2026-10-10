import { HttpClient } from '@angular/common/http';
import { Injector, runInInjectionContext } from '@angular/core';
import { of } from 'rxjs';
import { SILENCE_ERROR_TOAST } from 'src/app/core/request-context';
import { AttachmentApi } from './attachment.api.service';
import { MessageRecipientType } from 'src/app/message/constant/message-recipient-type.enum';

function buildApi(http: Partial<HttpClient>): AttachmentApi {
    const injector = Injector.create({ providers: [{ provide: HttpClient, useValue: http }] });
    return runInInjectionContext(injector, () => new AttachmentApi());
}

describe('AttachmentApi', () => {
    it('uploads the file and scope as multipart form data', () => {
        const post = vi.fn().mockReturnValue(of({}));
        const api = buildApi({ post });

        api.upload$(new File(['x'], 'a.txt'), {
            idMessageRecipientType: MessageRecipientType.team,
            idRecipient: 7
        }).subscribe();

        const [url, body] = post.mock.calls[0] as [string, FormData];
        expect(url).toBe('/api/private/attachment');
        expect(body.get('idMessageRecipientType')).toBe('2');
        expect(body.get('idRecipient')).toBe('7');
        expect((body.get('file') as File).name).toBe('a.txt');
    });

    it('downloads as a blob with the error toast silenced', () => {
        const get = vi.fn().mockReturnValue(of(new Blob()));
        const api = buildApi({ get });

        api.download$('abc').subscribe();

        const [url, options] = get.mock.calls[0] as [
            string,
            { responseType: string; context: { get: (token: unknown) => boolean } }
        ];
        expect(url).toBe('/api/private/attachment/abc');
        expect(options.responseType).toBe('blob');
        expect(options.context.get(SILENCE_ERROR_TOAST)).toBe(true);
    });
});
