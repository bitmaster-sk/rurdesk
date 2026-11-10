import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule, type FormGroup } from '@angular/forms';
import { Component, signal } from '@angular/core';
import {
    IconDeviceFloppy,
    IconLock,
    IconRefresh,
    IconUserCircle,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { TranslateModule } from '@ngx-translate/core';
import { page } from 'vitest/browser';
import { of } from 'rxjs';
import { AuthApi } from 'src/app/auth/api/auth.api.service';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AvatarComponent } from 'src/app/shared/avatar/avatar.component';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { UiModule } from 'src/app/ui/ui.module';
import { UserSettingsPage } from './user-settings.page';
import 'src/styles.scss';
import 'src/tailwind.css';

describe('UserSettingsPage — avatar colour (browser)', () => {
    const baseUser = {
        idUser: 1,
        name: 'tester',
        email: 't@t.sk',
        colorAvatarBg: '#112233',
        isAgent: false,
        isAdmin: false
    };

    let updateUser$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        updateUser$ = vi.fn((name: string, colorAvatarBg?: string) =>
            of({ ...baseUser, name, colorAvatarBg: colorAvatarBg ?? baseUser.colorAvatarBg })
        );
        await TestBed.configureTestingModule({
            declarations: [UserSettingsPage],
            imports: [ReactiveFormsModule],
            providers: [
                {
                    provide: AuthStore,
                    useValue: { user: signal(baseUser), updateUser$ }
                },
                {
                    provide: ToastNotificationService,
                    useValue: { showError: () => {}, showSuccess: () => {} }
                }
            ]
        })
            .overrideComponent(UserSettingsPage, { set: { template: '' } })
            .compileComponents();
    });

    function build(): UserSettingsPage {
        const fixture = TestBed.createComponent(UserSettingsPage);
        fixture.detectChanges(); // ngOnInit builds the form + subscriptions
        return fixture.componentInstance;
    }

    it('seeds the colour control and preview from the current user', () => {
        const comp = build();
        const form = (comp as unknown as { profileForm: FormGroup }).profileForm;
        expect(form.get('colorAvatarBg')!.value).toBe('#112233');
        expect(comp.avatarColor()).toBe('#112233');
    });

    it('shuffle sets a fresh hex on the control and updates the live preview', () => {
        const comp = build();
        const form = (comp as unknown as { profileForm: FormGroup }).profileForm;
        comp.onShuffleColor();
        const value = form.get('colorAvatarBg')!.value;
        expect(value).toMatch(/^#[0-9a-f]{6}$/);
        expect(comp.avatarColor()).toBe(value);
    });

    it('saving sends both the name and the chosen colour', () => {
        const comp = build();
        const form = (comp as unknown as { profileForm: FormGroup }).profileForm;
        form.get('colorAvatarBg')!.setValue('#abcdef');
        comp.onSaveUser();
        expect(updateUser$).toHaveBeenCalledWith('tester', '#abcdef');
    });

    it('a colour save shows the chip on the colour control, not the name input', () => {
        const comp = build();
        comp.onSaveUser('color');
        expect(comp.colorSaveStatus()).toBe(UiSaveState.Saved);
        expect(comp.nameSaveStatus()).toBe(UiSaveState.Idle);
    });

    it('a name save shows the chip on the name input, not the colour control', () => {
        const comp = build();
        comp.onSaveUser('name');
        expect(comp.nameSaveStatus()).toBe(UiSaveState.Saved);
        expect(comp.colorSaveStatus()).toBe(UiSaveState.Idle);
    });
});

@Component({ selector: 'app-user-api-keys', template: '', standalone: true })
class UserApiKeysStub {}

describe('UserSettingsPage layout (browser)', () => {
    const narrowViewports = [320, 375, 430];
    let fixture: ComponentFixture<UserSettingsPage>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [UserSettingsPage, AvatarComponent],
            imports: [
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconComponent,
                UserApiKeysStub
            ],
            providers: [
                {
                    provide: AuthStore,
                    useValue: {
                        user: signal({
                            idUser: 1,
                            name: 'John Snow',
                            email: 'john.snow.with.a.very.long.address@example.com',
                            colorAvatarBg: '#e5b48c',
                            isAgent: false,
                            isAdmin: false
                        }),
                        updateUser$: vi.fn()
                    }
                },
                { provide: AuthApi, useValue: {} },
                {
                    provide: ToastNotificationService,
                    useValue: { showError: vi.fn(), showSuccess: vi.fn() }
                },
                provideTablerIcons({ IconDeviceFloppy, IconLock, IconRefresh, IconUserCircle })
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(UserSettingsPage);
        fixture.detectChanges();
    });

    afterEach(() => fixture.destroy());

    function el(testid: string): HTMLElement {
        return (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(
            `[data-testid="${testid}"]`
        )!;
    }

    function input(id: string): HTMLElement {
        return (fixture.nativeElement as HTMLElement).querySelector<HTMLElement>(`#${id}`)!;
    }

    async function resize(width: number): Promise<void> {
        await page.viewport(width, 800);
        fixture.detectChanges();
        await fixture.whenStable();
    }

    it('stacks the profile and password fields inside the page at 320/375/430px', async () => {
        for (const width of narrowViewports) {
            await resize(width);

            expect(
                document.documentElement.scrollWidth,
                `no page overflow at ${width}px`
            ).toBeLessThanOrEqual(width + 1);
            for (const body of ['profile-body', 'password-body']) {
                expect(getComputedStyle(el(body)).flexDirection, `${body} at ${width}px`).toBe(
                    'column'
                );
            }
            for (const id of ['prof-name', 'prof-email', 'prof-color', 'pwd-current', 'pwd-new']) {
                expect(
                    input(id).getBoundingClientRect().right,
                    `${id} fits at ${width}px`
                ).toBeLessThanOrEqual(width + 1);
            }
        }
    });

    it('lines up the profile inputs on desktop even though the email has a hint below it', async () => {
        await resize(1280);

        expect(getComputedStyle(el('profile-body')).flexDirection).toBe('row');
        const nameTop = input('prof-name').getBoundingClientRect().top;
        expect(input('prof-email').getBoundingClientRect().top).toBeCloseTo(nameTop, 0);
        expect(input('prof-color').getBoundingClientRect().top).toBeCloseTo(nameTop, 0);
        expect(el('profile-avatar').getBoundingClientRect().top).toBeLessThan(nameTop);
    });
});
