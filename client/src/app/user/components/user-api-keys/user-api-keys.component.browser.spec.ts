import { Directive, forwardRef, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
    ControlValueAccessor,
    NG_VALUE_ACCESSOR,
    ReactiveFormsModule,
    FormsModule
} from '@angular/forms';
import {
    IconAlertTriangle,
    IconCircleX,
    IconCopy,
    IconKey,
    IconPlus,
    IconRefresh,
    IconTrash,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { page } from 'vitest/browser';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { UiModule } from 'src/app/ui/ui.module';
import { TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
import { UserApiKeyApi } from '../../api/user-api-key.api.service';
import { UserApiKey } from '../../model/user-api-key.model';
import { UserApiKeysComponent } from './user-api-keys.component';
import 'flatpickr/dist/flatpickr.css';
import 'src/styles.scss';
import 'src/tailwind.css';

@Directive({ selector: 'input[uiInput]', standalone: true })
class UiInputStub {
    public readonly invalid = input<boolean>(false);
}

@Directive({
    selector: 'input[uiDatepicker]',
    standalone: true,
    providers: [
        { provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => UiDatepickerStub), multi: true }
    ]
})
class UiDatepickerStub implements ControlValueAccessor {
    public readonly invalid = input<boolean>(false);
    public writeValue(): void {}
    public registerOnChange(): void {}
    public registerOnTouched(): void {}
}

@Directive({ selector: '[uiConfirm]', standalone: true })
class UiConfirmStub {
    public readonly confirmText = input<string>('');
    public readonly confirmed = output<void>();
}

const existing: UserApiKey = {
    idApiKey: 1,
    idUser: 7,
    name: 'laptop',
    createdAt: '2026-08-01T10:00:00Z',
    expiresAt: null,
    lastUsedAt: null
};

describe('UserApiKeysComponent (browser)', () => {
    let load$: ReturnType<typeof vi.fn>;
    let insert$: ReturnType<typeof vi.fn>;
    let regenerate$: ReturnType<typeof vi.fn>;
    let revoke$: ReturnType<typeof vi.fn>;
    let copy: ReturnType<typeof vi.fn>;
    let showError: ReturnType<typeof vi.fn>;
    let showSuccess: ReturnType<typeof vi.fn>;
    let keyLimit: number;
    let fixture: ComponentFixture<UserApiKeysComponent>;

    beforeEach(async () => {
        keyLimit = 10;
        load$ = vi.fn().mockReturnValue(of([existing]));
        insert$ = vi
            .fn()
            .mockReturnValue(of({ ...existing, idApiKey: 2, name: 'ci', rawKey: 'raw-secret' }));
        regenerate$ = vi.fn().mockReturnValue(of({ ...existing, rawKey: 'rotated-secret' }));
        revoke$ = vi.fn().mockReturnValue(of(undefined));
        copy = vi.fn().mockResolvedValue(true);
        showError = vi.fn();
        showSuccess = vi.fn();

        await TestBed.configureTestingModule({
            declarations: [UserApiKeysComponent],
            imports: [
                ReactiveFormsModule,
                FormsModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                UiButtonStub,
                UiTooltipStub,
                UiInputStub,
                UiDatepickerStub,
                UiConfirmStub
            ],
            providers: [
                {
                    provide: UserApiKeyApi,
                    useValue: { load$, insert$, regenerate$, revoke$ }
                },
                { provide: SettingsStore, useValue: { userApiKeyLimit: () => keyLimit } },
                { provide: ClipboardService, useValue: { copy } },
                { provide: ToastNotificationService, useValue: { showError, showSuccess } }
            ]
        }).compileComponents();
    });

    function render(): HTMLElement {
        fixture = TestBed.createComponent(UserApiKeysComponent);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function el(host: HTMLElement, testid: string): HTMLElement | null {
        return host.querySelector(`[data-testid="${testid}"]`);
    }

    function typeName(host: HTMLElement, value: string): void {
        const input = el(host, 'user-api-key-name') as HTMLInputElement;
        input.value = value;
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
    }

    function submit(host: HTMLElement): void {
        host.querySelector('form')!.dispatchEvent(new Event('submit'));
        fixture.detectChanges();
    }

    function fire(
        host: HTMLElement,
        testid: string,
        event: 'click' | 'confirmed',
        index = 0
    ): void {
        const target = host.querySelectorAll(`[data-testid="${testid}"]`)[index];
        if (event === 'click') {
            target.dispatchEvent(new Event('click'));
        } else {
            fixture.debugElement
                .query(node => node.nativeElement === target)
                .injector.get(UiConfirmStub)
                .confirmed.emit();
        }
        fixture.detectChanges();
    }

    it('lists the keys the user already has', () => {
        const host = render();

        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(1);
        expect(el(host, 'user-api-key-table')!.textContent).toContain('laptop');
        expect(el(host, 'user-api-key-count')!.textContent).toContain('1 / 10');
        expect(el(host, 'user-api-key-empty')).toBeNull();
    });

    it('shows an empty state, not a table, when the user has no keys', () => {
        load$.mockReturnValue(of([]));
        const host = render();

        expect(el(host, 'user-api-key-empty')).not.toBeNull();
        expect(el(host, 'user-api-key-table')).toBeNull();
    });

    it('offers a retry instead of the empty state when loading fails', () => {
        load$.mockReturnValue(throwError(() => new Error('down')));
        const host = render();

        expect(el(host, 'user-api-key-load-failed')).not.toBeNull();
        expect(el(host, 'user-api-key-empty')).toBeNull();
    });

    it('creating a key reveals the raw value once and adds the row', () => {
        const host = render();
        typeName(host, 'ci');
        submit(host);

        expect(insert$).toHaveBeenCalledWith({ name: 'ci', expiresAt: null });
        expect(el(host, 'user-api-key-revealed')!.textContent).toContain('raw-secret');
        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(2);
        expect(el(host, 'user-api-key-table')!.textContent).not.toContain('raw-secret');
    });

    it('refuses to submit a whitespace-only name', () => {
        const host = render();
        typeName(host, '   ');
        submit(host);

        expect(insert$).not.toHaveBeenCalled();
    });

    it('disables creation and explains why once the limit is reached', () => {
        keyLimit = 1;
        const host = render();
        typeName(host, 'ci');

        expect(el(host, 'user-api-key-limit-reached')).not.toBeNull();
        submit(host);
        expect(insert$).not.toHaveBeenCalled();
    });

    it('resyncs the list when the server rejects a create', () => {
        insert$.mockReturnValue(throwError(() => new Error('409')));
        const host = render();
        typeName(host, 'ci');
        submit(host);

        expect(load$).toHaveBeenCalledTimes(2);
        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(1);
    });

    it('regenerating replaces the row in place and reveals the new key', () => {
        const host = render();
        fire(host, 'user-api-key-regenerate', 'confirmed');

        expect(regenerate$).toHaveBeenCalledWith(1);
        expect(el(host, 'user-api-key-revealed')!.textContent).toContain('rotated-secret');
        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(1);
        expect(el(host, 'user-api-key-table')!.textContent).not.toContain('rotated-secret');
    });

    it('keeps the row when regenerating fails', () => {
        regenerate$.mockReturnValue(throwError(() => new Error('nope')));
        const host = render();
        fire(host, 'user-api-key-regenerate', 'confirmed');

        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(1);
        expect(el(host, 'user-api-key-revealed')).toBeNull();
    });

    it('revoking removes the row', () => {
        const host = render();
        fire(host, 'user-api-key-revoke', 'confirmed');

        expect(revoke$).toHaveBeenCalledWith(1);
        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(0);
    });

    it('keeps the row when revoking fails', () => {
        revoke$.mockReturnValue(throwError(() => new Error('nope')));
        const host = render();
        fire(host, 'user-api-key-revoke', 'confirmed');

        expect(host.querySelectorAll('[data-testid="user-api-key-row"]')).toHaveLength(1);
    });

    it('revoking another key does not hide a freshly revealed value', () => {
        const host = render();
        typeName(host, 'ci');
        submit(host);
        // Index 1 is the pre-existing key; the revealed value belongs to index 0.
        fire(host, 'user-api-key-revoke', 'confirmed', 1);

        expect(revoke$).toHaveBeenCalledWith(1);
        expect(el(host, 'user-api-key-revealed')!.textContent).toContain('raw-secret');
    });

    it('revoking the key whose value is on screen hides that value', () => {
        const host = render();
        typeName(host, 'ci');
        submit(host);
        fire(host, 'user-api-key-revoke', 'confirmed');

        expect(revoke$).toHaveBeenCalledWith(2);
        expect(el(host, 'user-api-key-revealed')).toBeNull();
    });

    it('copies the revealed value and confirms it', async () => {
        const host = render();
        typeName(host, 'ci');
        submit(host);
        fire(host, 'user-api-key-copy', 'click');
        await Promise.resolve();

        expect(copy).toHaveBeenCalledWith('raw-secret');
        expect(showSuccess).toHaveBeenCalledWith('USER_API_KEY.COPIED');
    });

    it('tells the user to copy manually when the clipboard is unavailable', async () => {
        copy.mockResolvedValue(false);
        const host = render();
        typeName(host, 'ci');
        submit(host);
        fire(host, 'user-api-key-copy', 'click');
        await Promise.resolve();
        await Promise.resolve();

        expect(showError).toHaveBeenCalledWith('USER_API_KEY.COPY_FAILED');
    });
});

interface Viewport {
    readonly width: number;
    readonly height: number;
}

const narrowViewports: Viewport[] = [
    { width: 320, height: 800 },
    { width: 375, height: 800 },
    { width: 430, height: 800 }
];

const longName = 'CI pipeline for the very long integration suite on every branch';
const longSecret = 'rk_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';

describe('UserApiKeysComponent layout (browser)', () => {
    let fixture: ComponentFixture<UserApiKeysComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [UserApiKeysComponent],
            imports: [
                ReactiveFormsModule,
                FormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent
            ],
            providers: [
                {
                    provide: UserApiKeyApi,
                    useValue: {
                        load$: vi.fn().mockReturnValue(of([existing])),
                        insert$: vi
                            .fn()
                            .mockReturnValue(
                                of({ ...existing, idApiKey: 2, name: 'ci', rawKey: longSecret })
                            ),
                        regenerate$: vi.fn(),
                        revoke$: vi.fn()
                    }
                },
                { provide: SettingsStore, useValue: { userApiKeyLimit: () => 10 } },
                { provide: ClipboardService, useValue: { copy: vi.fn() } },
                {
                    provide: ToastNotificationService,
                    useValue: { showError: vi.fn(), showSuccess: vi.fn() }
                },
                provideTablerIcons({
                    IconAlertTriangle,
                    IconCircleX,
                    IconCopy,
                    IconKey,
                    IconPlus,
                    IconRefresh,
                    IconTrash
                })
            ]
        }).compileComponents();
    });

    afterEach(() => fixture.destroy());

    function render(): HTMLElement {
        fixture = TestBed.createComponent(UserApiKeysComponent);
        fixture.detectChanges();
        return fixture.nativeElement as HTMLElement;
    }

    function el(host: HTMLElement, testid: string): HTMLElement {
        return host.querySelector<HTMLElement>(`[data-testid="${testid}"]`)!;
    }

    function createKey(host: HTMLElement): void {
        const name = el(host, 'user-api-key-name') as HTMLInputElement;
        name.value = longName;
        name.dispatchEvent(new Event('input'));
        fixture.detectChanges();
        host.querySelector('form')!.dispatchEvent(new Event('submit'));
        fixture.detectChanges();
    }

    async function resize(viewport: Viewport): Promise<void> {
        await page.viewport(viewport.width, viewport.height);
        fixture.detectChanges();
        await fixture.whenStable();
    }

    it('keeps the create form and the one-time secret inside the page at 320/375/430px', async () => {
        const host = render();
        createKey(host);

        const form = host.querySelector('form')!;
        const revealed = el(host, 'user-api-key-revealed');
        const secretCode = revealed.querySelector('.secret-code')!;
        const createBtn = el(host, 'user-api-key-create');

        for (const viewport of narrowViewports) {
            await resize(viewport);

            expect(
                document.documentElement.scrollWidth,
                `no page overflow at ${viewport.width}px`
            ).toBeLessThanOrEqual(viewport.width + 1);
            for (const node of [host, form, revealed, secretCode]) {
                expect(
                    node.getBoundingClientRect().right,
                    `block fits at ${viewport.width}px`
                ).toBeLessThanOrEqual(viewport.width + 1);
            }
            expect(getComputedStyle(form).flexDirection, `form stacks at ${viewport.width}px`).toBe(
                'column'
            );
            expect(
                createBtn.getBoundingClientRect().width,
                `create is full width at ${viewport.width}px`
            ).toBeGreaterThanOrEqual(viewport.width - 64);
        }
    });

    it('lets only the table scroll horizontally while all its columns and actions stay reachable', async () => {
        const host = render();

        const scroll = el(host, 'user-api-key-table-scroll');
        const table = el(host, 'user-api-key-table');
        const revoke = el(host, 'user-api-key-revoke');

        for (const viewport of narrowViewports) {
            await resize(viewport);

            expect(
                document.documentElement.scrollWidth,
                `no page overflow at ${viewport.width}px`
            ).toBeLessThanOrEqual(viewport.width + 1);
            expect(getComputedStyle(scroll).overflowX).toBe('auto');
            expect(table.querySelectorAll('thead th')).toHaveLength(5);

            scroll.scrollLeft = scroll.scrollWidth;
            const revokeRect = revoke.getBoundingClientRect();
            expect(revokeRect.width, `revoke rendered at ${viewport.width}px`).toBeGreaterThan(0);
            expect(revokeRect.right, `revoke reachable at ${viewport.width}px`).toBeLessThanOrEqual(
                scroll.getBoundingClientRect().right + 1
            );
        }
    });

    it('opens the datepicker on a phone and keeps the picked day', async () => {
        const host = render();
        await resize({ width: 320, height: 800 });

        const expires = el(host, 'user-api-key-expires') as HTMLInputElement;
        const calendar = await vi.waitFor(() => {
            const node = document.querySelector<HTMLElement>('.flatpickr-calendar');
            expect(node).not.toBeNull();
            return node!;
        });

        expires.click();
        await vi.waitFor(() => expect(calendar.classList).toContain('open'));
        expect(calendar.getBoundingClientRect().right).toBeLessThanOrEqual(321);

        const day = Array.from(
            calendar.querySelectorAll<HTMLElement>(
                '.flatpickr-day:not(.prevMonthDay):not(.nextMonthDay):not(.flatpickr-disabled)'
            )
        ).at(-1)!;
        day.click();
        fixture.detectChanges();

        const picked = (
            fixture.componentInstance as unknown as {
                form: { value: { expiresAt: Date | null } };
            }
        ).form.value.expiresAt;
        expect(picked).toBeInstanceOf(Date);
        expect(picked!.getDate()).toBe(Number(day.textContent));
    });

    it('keeps the create form on a single bottom-aligned row on desktop', async () => {
        const host = render();
        await resize({ width: 1280, height: 900 });

        const form = host.querySelector('form')!;
        expect(getComputedStyle(form).flexDirection).toBe('row');
        expect(el(host, 'user-api-key-create').getBoundingClientRect().bottom).toBeCloseTo(
            el(host, 'user-api-key-name').getBoundingClientRect().bottom,
            0
        );
    });
});
