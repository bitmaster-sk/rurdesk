import { Injector, runInInjectionContext } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { of } from 'rxjs';
import { IssueParticipantApi } from './issue-participant.api.service';
import { IssueParticipantModel, ParticipantSource } from '../model/issue-participant.model';

const mockParticipant: IssueParticipantModel = {
    idUser: 1,
    name: 'Alice',
    colorAvatarBg: '#aabbcc',
    isBot: false,
    source: ParticipantSource.Creator,
    hasNotificationsEnabled: true
};

describe('IssueParticipantApi', () => {
    let get: ReturnType<typeof vi.fn>;
    let post: ReturnType<typeof vi.fn>;
    let patch: ReturnType<typeof vi.fn>;
    let service: IssueParticipantApi;

    beforeEach(() => {
        get = vi.fn().mockReturnValue(of([mockParticipant]));
        post = vi.fn().mockReturnValue(of(undefined));
        patch = vi.fn().mockReturnValue(of(undefined));
        const injector = Injector.create({
            providers: [{ provide: HttpClient, useValue: { get, post, patch } }]
        });
        service = runInInjectionContext(injector, () => new IssueParticipantApi());
    });

    it('list$ GETs /api/private/project/:idProject/issue/:idIssuePublic/participant', () => {
        let res: IssueParticipantModel[] | undefined;
        service.list$(10, 42).subscribe(r => (res = r));

        expect(get).toHaveBeenCalledWith('/api/private/project/10/issue/42/participant');
        expect(res).toEqual([mockParticipant]);
    });

    it('add$ POSTs to /api/private/project/:idProject/issue/:idIssuePublic/participant with idUser', () => {
        service.add$(10, 42, 7).subscribe();

        expect(post).toHaveBeenCalledWith('/api/private/project/10/issue/42/participant', {
            idUser: 7
        });
    });

    it('setNotifications$ PATCHes .../participant/notifications with enabled', () => {
        service.setNotifications$(10, 42, false).subscribe();

        expect(patch).toHaveBeenCalledWith(
            '/api/private/project/10/issue/42/participant/notifications',
            { enabled: false }
        );
    });
});
