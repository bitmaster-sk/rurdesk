import { Directive, forwardRef, input, output } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import {
    ControlValueAccessor,
    NG_VALUE_ACCESSOR,
    ReactiveFormsModule,
    FormsModule
} from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of, throwError } from 'rxjs';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardService } from 'src/app/core/clipboard.service';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { TablerIconStub, UiButtonStub, UiTooltipStub } from 'src/testing/stubs';
import { UserApiKeyApi } from '../../api/user-api-key.api.service';
import { UserApiKey } from '../../model/user-api-key.model';
import { UserApiKeysComponent } from './user-api-keys.component';

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

// ─── Mobile layout (issue #253) ─────────────────────────────────────────────
// Real ui-* children (UiModule), not stubs: the assertions are about layout,
// so the buttons/datepicker/table chrome must render for real. Viewports are
// the DoD widths 320/375/430.

import { page } from '@vitest/browser/context';
import { UiModule } from 'src/app/ui/ui.module';
// The browser config has no global stylesheet: Angular only injects
// component styles. These layout assertions need the real ones (Tailwind
// utilities + .panel/.ui-table/.ui-table-scroll/.secret-code), exactly like
// the split-pane spec imports its own stylesheet.
import 'src/styles.scss';
import 'src/tailwind.css';
// flatpickr's base stylesheet is an angular.json global style, not part of
// styles.scss — the datepicker test needs it so the popup is position:absolute
// and sized like in the app.
import 'flatpickr/dist/flatpickr.css';
import {
    IconAlertTriangle,
    IconCircleX,
    IconCopy,
    IconPlus,
    IconRefresh,
    IconTrash
} from '@tabler/icons-angular';
import { provideTablerIcons } from '@tabler/icons-angular';

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
    let load$: ReturnType<typeof vi.fn>;
    let insert$: ReturnType<typeof vi.fn>;
    let regenerate$: ReturnType<typeof vi.fn>;
    let revoke$: ReturnType<typeof vi.fn>;
    let copy: ReturnType<typeof vi.fn>;
    let fixture: ComponentFixture<UserApiKeysComponent>;

    beforeEach(async () => {
        load$ = vi.fn().mockReturnValue(of([existing]));
        insert$ = vi
            .fn()
            .mockReturnValue(of({ ...existing, idApiKey: 2, name: 'ci', rawKey: longSecret }));
        regenerate$ = vi.fn().mockReturnValue(of({ ...existing, rawKey: 'rotated-secret' }));
        revoke$ = vi.fn().mockReturnValue(of(undefined));
        copy = vi.fn().mockResolvedValue(true);

        await TestBed.configureTestingModule({
            declarations: [UserApiKeysComponent],
            imports: [ReactiveFormsModule, FormsModule, TranslateModule.forRoot(), UiModule],
            providers: [
                {
                    provide: UserApiKeyApi,
                    useValue: { load$, insert$, regenerate$, revoke$ }
                },
                { provide: SettingsStore, useValue: { userApiKeyLimit: () => 10 } },
                { provide: ClipboardService, useValue: { copy } },
                {
                    provide: ToastNotificationService,
                    useValue: { showError: vi.fn(), showSuccess: vi.fn() }
                },
                provideTablerIcons({
                    IconAlertTriangle,
                    IconCircleX,
                    IconCopy,
                    IconPlus,
                    IconRefresh,
                    IconTrash
                })
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

    function createKey(host: HTMLElement): void {
        const input = el(host, 'user-api-key-name') as HTMLInputElement;
        input.value = longName;
        input.dispatchEvent(new Event('input'));
        fixture.detectChanges();
        host.querySelector('form')!.dispatchEvent(new Event('submit'));
        fixture.detectChanges();
    }

    async function resize(viewport: Viewport): Promise<void> {
        await page.viewport(viewport.width, viewport.height);
    }

    it('keeps the panel, the create form and the one-time secret inside the page at 320/375/430px', async () => {
        const host = render();
        createKey(host); // long name + long secret, so the revealed block is at its widest

        const form = host.querySelector('form')!;
        const revealed = el(host, 'user-api-key-revealed')!;
        const secretCode = revealed.querySelector('.secret-code')!;

        for (const viewport of narrowViewports) {
            await resize(viewport);
            fixture.detectChanges();
            await fixture.whenStable();

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

            // Fields and Create stack full-width on mobile (column direction).
            const createBtn = el(host, 'user-api-key-create')!;
            expect(getComputedStyle(form).flexDirection, `form stacks at ${viewport.width}px`).toBe(
                'column'
            );
            expect(
                createBtn.getBoundingClientRect().width,
                `create is full width at ${viewport.width}px`
            ).toBeGreaterThanOrEqual(viewport.width - 64); // p-4 on both sides
        }
    });

    it('lets only the table scroll horizontally while all its actions stay reachable', async () => {
        const host = render();

        const scroll = el(host, 'user-api-key-table-scroll')!;
        const table = el(host, 'user-api-key-table')!;

        for (const viewport of narrowViewports) {
            await resize(viewport);
            fixture.detectChanges();
            await fixture.whenStable();

            expect(
                document.documentElement.scrollWidth,
                `no page overflow at ${viewport.width}px`
            ).toBeLessThanOrEqual(viewport.width + 1);

            // The table is wider than its wrapper → the wrapper is the scroller.
            expect(
                scroll.scrollWidth,
                `table overflows its wrapper at ${viewport.width}px`
            ).toBeGreaterThan(scroll.clientWidth - 1);
            expect(getComputedStyle(scroll).overflowX).toBe('auto');

            // All five columns are rendered (nothing is hidden to fit).
            expect(table.querySelectorAll('thead th')).toHaveLength(5);

            // Every action button is inside the scrollable area and reachable
            // by scrolling the wrapper to the end.
            const revoke = el(host, 'user-api-key-revoke')!;
            scroll.scrollLeft = scroll.scrollWidth;
            fixture.detectChanges();
            const revokeRect = revoke.getBoundingClientRect();
            expect(revokeRect.right, `revoke reachable at ${viewport.width}px`).toBeLessThanOrEqual(
                scroll.getBoundingClientRect().right + 1
            );
            expect(revokeRect.width, `revoke rendered at ${viewport.width}px`).toBeGreaterThan(0);
        }
    });

    it('opens the real datepicker, keeps the picked date and keeps desktop as a single row', async () => {
        const host = render();
        await resize({ width: 1280, height: 900 });
        fixture.detectChanges();

        const expires = el(host, 'user-api-key-expires') as HTMLInputElement;
        // Wait for the lazily loaded flatpickr to attach.
        for (
            let i = 0;
            i < 100 && !(expires as unknown as { _flatpickr?: unknown })._flatpickr;
            i++
        ) {
            await new Promise(r => setTimeout(r, 10));
        }
        const fp = (
            expires as unknown as {
                _flatpickr: {
                    open: () => void;
                    calendarContainer: HTMLElement;
                    setDate: (d: Date, trigger?: boolean) => void;
                };
            }
        )._flatpickr;
        fp.open();
        await new Promise(r => setTimeout(r, 0));
        expect(
            document.body.contains(fp.calendarContainer) || fp.calendarContainer.isConnected,
            'datepicker calendar opened'
        ).toBe(true);

        const picked = new Date(2027, 0, 15);
        fp.setDate(picked, true);
        await fixture.whenStable();
        fixture.detectChanges();
        const component = fixture.componentInstance as unknown as {
            form: { value: { expiresAt: Date | null } };
        };
        expect(component.form.value.expiresAt).toBeInstanceOf(Date);

        // Desktop: the form stays one row aligned at the bottom.
        const form = host.querySelector('form')!;
        expect(getComputedStyle(form).flexDirection).toBe('row');
        const name = el(host, 'user-api-key-name')!;
        const createBtn = el(host, 'user-api-key-create')!;
        expect(createBtn.getBoundingClientRect().bottom).toBeCloseTo(
            name.getBoundingClientRect().bottom,
            0
        );
    });
});
