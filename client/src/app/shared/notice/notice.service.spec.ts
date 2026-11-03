// @vitest-environment jsdom
import { PlatformLocation } from '@angular/common';
import { Injector, runInInjectionContext } from '@angular/core';
import { AuthTokenStore } from 'src/app/auth/store/auth-token.store';
import { NoticeService } from './notice.service';

function buildService(location: PlatformLocation, tokenStore: AuthTokenStore): NoticeService {
    const injector = Injector.create({
        providers: [
            { provide: PlatformLocation, useValue: location },
            { provide: AuthTokenStore, useValue: tokenStore }
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
    const tokenStore = { getToken: () => token } as unknown as AuthTokenStore;
    const service = buildService(location, tokenStore);
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
                public url: string;
                public protocols?: string | string[];
                public constructor(url: string, protocols?: string | string[]) {
                    this.url = url;
                    this.protocols = protocols;
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
        const tokenStore = { getToken: () => token } as unknown as AuthTokenStore;
        buildService(location, tokenStore);
        expect(sockets).toHaveLength(0);

        token = 'jwt-after-login';
        vi.advanceTimersByTime(60 * 1000);

        expect(sockets).toHaveLength(1);
        expect(sockets[0].protocols).toEqual(['Authorization', 'jwt-after-login']);
    });
});

class ListeningSocket {
    public static created: ListeningSocket[] = [];
    public readonly protocols: string | string[] | undefined;
    public close = vi.fn();
    private readonly listeners = new Map<string, ((evt: unknown) => void)[]>();

    public constructor(_url: string, protocols?: string | string[]) {
        this.protocols = protocols;
        ListeningSocket.created.push(this);
    }

    public addEventListener(type: string, listener: (evt: unknown) => void): void {
        this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
    }

    public fire(type: string, evt: unknown = {}): void {
        for (const listener of this.listeners.get(type) ?? []) listener(evt);
    }
}

function buildListening(getToken: () => string | null): NoticeService {
    ListeningSocket.created = [];
    vi.stubGlobal('WebSocket', ListeningSocket);
    const location = {
        protocol: 'http:',
        hostname: 'localhost',
        port: '9000'
    } as unknown as PlatformLocation;
    return buildService(location, { getToken } as unknown as AuthTokenStore);
}

function messageNotice(): MessageEvent {
    return new MessageEvent('message', {
        data: JSON.stringify({ subject: 'message', action: 'c', payload: {} })
    });
}

describe('NoticeService.reconnect', () => {
    it('closes the old socket and opens one with the current token', () => {
        let token: string | null = 'jwt-john';
        const service = buildListening(() => token);
        const [old] = ListeningSocket.created;

        token = 'jwt-gone';
        service.reconnect();

        expect(old.close).toHaveBeenCalled();
        expect(ListeningSocket.created).toHaveLength(2);
        expect(ListeningSocket.created[1].protocols).toEqual(['Authorization', 'jwt-gone']);
    });

    it('ignores notices still arriving on the old socket', () => {
        const service = buildListening(() => 'jwt');
        const received = vi.fn();
        service.Message.subscribe(received);
        const [old] = ListeningSocket.created;

        service.reconnect();
        old.fire('message', messageNotice());

        expect(received).not.toHaveBeenCalled();

        ListeningSocket.created[1].fire('message', messageNotice());

        expect(received).toHaveBeenCalledTimes(1);
    });

    it('does not open a second socket when the old one finishes closing', () => {
        vi.useFakeTimers();
        const service = buildListening(() => 'jwt');
        const [old] = ListeningSocket.created;

        service.reconnect();
        old.fire('close');
        vi.advanceTimersByTime(60 * 1000);

        expect(ListeningSocket.created).toHaveLength(2);
    });

    it('stays disconnected after logout and connects once a token appears', () => {
        vi.useFakeTimers();
        let token: string | null = 'jwt-john';
        const service = buildListening(() => token);

        token = null;
        service.reconnect();

        expect(ListeningSocket.created).toHaveLength(1);

        token = 'jwt-gone';
        service.reconnect();
        vi.advanceTimersByTime(60 * 1000);

        expect(ListeningSocket.created).toHaveLength(2);
        expect(ListeningSocket.created[1].protocols).toEqual(['Authorization', 'jwt-gone']);
    });
});
