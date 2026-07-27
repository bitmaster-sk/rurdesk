import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule, type FormGroup } from '@angular/forms';
import { BehaviorSubject, of } from 'rxjs';
import { UserService } from 'src/app/auth/user.service';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { UiSaveState } from 'src/app/ui/components/save-status/save-status-chip.component';
import { UserSettingsPage } from './user-settings.page';

describe('UserSettingsPage — avatar colour (browser)', () => {
    const baseUser = {
        idUser: 1,
        name: 'tester',
        email: 't@t.sk',
        colorAvatarBg: '#112233',
        isBot: false,
        isAdmin: false
    };

    let updateUser: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        updateUser = vi.fn((name: string, colorAvatarBg?: string) =>
            of({ ...baseUser, name, colorAvatarBg: colorAvatarBg ?? baseUser.colorAvatarBg })
        );
        await TestBed.configureTestingModule({
            declarations: [UserSettingsPage],
            imports: [ReactiveFormsModule],
            providers: [
                {
                    provide: UserService,
                    useValue: { user: new BehaviorSubject(baseUser), updateUser }
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
        expect(updateUser).toHaveBeenCalledWith('tester', '#abcdef');
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
