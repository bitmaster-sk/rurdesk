import { TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { page } from '@vitest/browser/context';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthApi } from '../api/auth.api.service';
import { AuthModule } from '../auth.module';
import { LoginComponent } from '../login/login.component';
import { SessionService } from '../service/session.service';
import { AuthTokenStore } from '../store/auth-token.store';
import { AuthPage } from './auth.page';

interface Viewport {
    readonly width: number;
    readonly height: number;
}

const viewports: Viewport[] = [
    { width: 320, height: 667 },
    { width: 375, height: 667 },
    { width: 430, height: 932 }
];

describe('AuthPage (browser)', () => {
    afterEach(() => TestBed.resetTestingModule());

    async function render(path: 'login' | 'register') {
        const authApi = {
            login$: vi.fn(),
            register$: vi.fn()
        };
        const tokenStore = { hasToken: vi.fn().mockReturnValue(false) };
        const session = { start: vi.fn(), end: vi.fn() };

        await TestBed.configureTestingModule({
            imports: [AuthModule, TranslateModule.forRoot(), RouterTestingModule],
            providers: [
                { provide: AuthApi, useValue: authApi },
                { provide: AuthTokenStore, useValue: tokenStore },
                { provide: SessionService, useValue: session },
                {
                    provide: ActivatedRoute,
                    useValue: { url: of([{ path }]) }
                }
            ]
        }).compileComponents();

        const fixture = TestBed.createComponent(AuthPage);
        fixture.detectChanges();
        await fixture.whenStable();
        fixture.detectChanges();
        return { fixture, authApi, tokenStore, session };
    }

    async function resize(viewport: Viewport) {
        await page.viewport(viewport.width, viewport.height);
    }

    function authPage(): HTMLElement {
        return document.querySelector('.auth-page')!;
    }

    function authCard(): HTMLElement {
        return document.querySelector('.auth-card')!;
    }

    function loginHost(): HTMLElement {
        return document.querySelector('.login')!;
    }

    function registerHost(): HTMLElement {
        return document.querySelector('.register')!;
    }

    it('has no horizontal overflow and the login uses a readable width on narrow viewports', async () => {
        const { fixture } = await render('login');

        for (const viewport of viewports) {
            await resize(viewport);
            fixture.detectChanges();
            await fixture.whenStable();

            expect(
                document.documentElement.scrollWidth,
                `no horizontal overflow at ${viewport.width}px`
            ).toBeLessThanOrEqual(viewport.width + 1);

            const card = authCard();
            expect(card.clientWidth, `card width at ${viewport.width}px`).toBeLessThanOrEqual(
                viewport.width
            );
            expect(card.clientWidth, `card width at ${viewport.width}px`).toBeGreaterThanOrEqual(
                280
            );

            const login = loginHost();
            expect(login.clientWidth, `login width at ${viewport.width}px`).toBeGreaterThanOrEqual(
                280
            );
            expect(
                login.getBoundingClientRect().right,
                `login fits at ${viewport.width}px`
            ).toBeLessThanOrEqual(viewport.width + 1);
        }
    });

    it('keeps the login content scrollable and the robot visible on a short landscape viewport with an error', async () => {
        const { fixture, authApi } = await render('login');
        await resize({ width: 667, height: 375 });
        fixture.detectChanges();
        await fixture.whenStable();

        const login = fixture.debugElement.query(By.directive(LoginComponent))
            .componentInstance as LoginComponent;
        authApi.login$.mockReturnValue(throwError(() => new Error('401')));
        login.form.setValue({
            email: 'a@b.sk',
            password: 'secret',
            hasExtendedSessionLifetime: false
        });
        login.onLogin();
        fixture.detectChanges();
        await fixture.whenStable();

        const pageEl = authPage();
        expect(
            pageEl.scrollHeight,
            'scroll container can hold the full content'
        ).toBeGreaterThanOrEqual(pageEl.clientHeight);
        expect(pageEl.scrollTop, 'top of the scroll container is reachable').toBe(0);

        const robot = document.querySelector('app-auth-robot svg') as SVGElement;
        expect(
            robot.getBoundingClientRect().top,
            'robot is not cut off at the top'
        ).toBeGreaterThanOrEqual(0);

        const error = document.querySelector('.login__error') as HTMLElement;
        expect(error, 'error message is rendered').not.toBeNull();
        error.scrollIntoView();
        expect(
            error.getBoundingClientRect().bottom,
            'error message is reachable by scrolling'
        ).toBeLessThanOrEqual(pageEl.clientHeight + 1);
    });

    it('keeps the centred 320 px desktop composition', async () => {
        const { fixture } = await render('login');
        await resize({ width: 1280, height: 800 });
        fixture.detectChanges();
        await fixture.whenStable();

        const card = authCard();
        expect(card.clientWidth, 'card is 320 px on desktop').toBe(320);

        const login = loginHost();
        const loginRect = login.getBoundingClientRect();
        const cardRect = card.getBoundingClientRect();
        expect(loginRect.left).toBeCloseTo(cardRect.left, 0);
        expect(loginRect.right).toBeCloseTo(cardRect.right, 0);
    });

    it('remains usable for registration in the same container', async () => {
        const { fixture } = await render('register');
        await resize({ width: 375, height: 667 });
        fixture.detectChanges();
        await fixture.whenStable();

        expect(
            document.documentElement.scrollWidth,
            'no horizontal overflow for register'
        ).toBeLessThanOrEqual(376);

        const register = registerHost();
        expect(register.clientWidth, 'register form is readable').toBeGreaterThanOrEqual(280);
    });
});
