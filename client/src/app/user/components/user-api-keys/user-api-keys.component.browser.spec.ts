import { Component, Directive, forwardRef, input, output } from '@angular/core';
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

@Component({ selector: 'ui-button', template: '<ng-content></ng-content>', standalone: true })
class UiButtonWithSeverityStub extends UiButtonStub {
    public readonly severity = input<string>('');
    public readonly variant = input<string>('');
    public readonly size = input<string>('');
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
                UiButtonWithSeverityStub,
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
