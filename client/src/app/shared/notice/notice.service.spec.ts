// @vitest-environment jsdom
import { PlatformLocation } from '@angular/common';
import { Injector, runInInjectionContext } from '@angular/core';
import { UserService } from 'src/app/auth/user.service';
import { NoticeService } from './notice.service';

function buildService(location: PlatformLocation, sUser: UserService): NoticeService {
    const injector = Injector.create({
        providers: [
            { provide: PlatformLocation, useValue: location },
            { provide: UserService, useValue: sUser }
        ]
    });
    return runInInjectionContext(injector, () => new NoticeService());
}

interface FakeSocket {
    url: string;
    protocols: string | string[] | undefined;
    addEventListener: ReturnType<typeof vi.fn>;
}

function build(token: string | null, protocol = 'http:', port = '9000') {
    const sockets: FakeSocket[] = [];
    vi.stubGlobal(
        'WebSocket',
        class {
            public url: string;
            public protocols: string | string[] | undefined;
            public addEventListener = vi.fn();
            public constructor(url: string, protocols?: string | string[]) {
                this.url = url;
                this.protocols = protocols;
                sockets.push(this);
            }
        }
    );

    const location = { protocol, hostname: 'localhost', port } as unknown as PlatformLocation;
    const sUser = { getAuthLocal: () => token } as unknown as UserService;
    const service = buildService(location, sUser);
    return { service, sockets };
}

afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
});

describe('NoticeService handshake', () => {
    // The browser WebSocket API cannot set an Authorization header, so the token
    // rides along as a subprotocol offer that the server selects.
    it('offers the token from localStorage as a subprotocol', () => {
        const { sockets } = build('jwt-123');

        expect(sockets).toHaveLength(1);
        expect(sockets[0].protocols).toEqual(['Authorization', 'jwt-123']);
    });

    it('builds a same-origin url for the ws endpoint', () => {
        const { sockets } = build('jwt-123');

        expect(sockets[0].url).toBe('ws://localhost:9000/api/private/ws');
    });

    it('uses wss when the page is served over https', () => {
        const { sockets } = build('jwt-123', 'https:', '443');

        expect(sockets[0].url).toBe('wss://localhost:443/api/private/ws');
    });

    // Anonymous visitors sit on the login page with no token; connecting there
    // only produced a guaranteed 401 on every retry.
    it('does not connect without a token', () => {
        const { sockets } = build(null);

        expect(sockets).toHaveLength(0);
    });

    it('retries once a token becomes available', () => {
        vi.useFakeTimers();
        const sockets: FakeSocket[] = [];
        vi.stubGlobal(
            'WebSocket',
            class {
                public addEventListener = vi.fn();
                constructor(
                    public url: string,
                    public protocols?: string | string[]
                ) {
                    sockets.push(this as unknown as FakeSocket);
                }
            }
        );

        let token: string | null = null;
        const location = {
            protocol: 'http:',
            hostname: 'localhost',
            port: '9000'
        } as unknown as PlatformLocation;
        const sUser = { getAuthLocal: () => token } as unknown as UserService;
        new NoticeService(location, sUser);
        expect(sockets).toHaveLength(0);

        token = 'jwt-after-login';
        vi.advanceTimersByTime(60 * 1000);

        expect(sockets).toHaveLength(1);
        expect(sockets[0].protocols).toEqual(['Authorization', 'jwt-after-login']);
    });
});
