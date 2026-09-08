import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule, type FormGroup } from '@angular/forms';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminUserApi } from '../../api/admin-user.api.service';
import { CreateUserDialogComponent } from './create-user-dialog.component';

describe('CreateUserDialogComponent — isAgent validator wiring (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [CreateUserDialogComponent],
            imports: [ReactiveFormsModule],
            providers: [
                { provide: AdminUserApi, useValue: {} },
                { provide: ToastNotificationService, useValue: {} }
            ]
        })
            // Skip the dialog template — we only exercise the form logic.
            .overrideComponent(CreateUserDialogComponent, { set: { template: '' } })
            .compileComponents();
    });

    function form(): FormGroup {
        const fixture = TestBed.createComponent(CreateUserDialogComponent);
        fixture.detectChanges();
        return (fixture.componentInstance as unknown as { form: FormGroup }).form;
    }

    it('turning isAgent on clears email requirement and makes gatewayUrl required', () => {
        const f = form();
        f.controls['isAgent'].setValue(true);
        expect(f.controls['email'].valid).toBe(true); // empty email OK for an agent
        expect(f.controls['gatewayUrl'].valid).toBe(false); // required for an agent
    });

    it('turning isAgent off requires email/password and clears gatewayUrl requirement', () => {
        const f = form();
        f.controls['isAgent'].setValue(true);
        f.controls['isAgent'].setValue(false);
        expect(f.controls['email'].valid).toBe(false); // required for a human
        expect(f.controls['password'].valid).toBe(false);
        expect(f.controls['gatewayUrl'].valid).toBe(true);
    });
});
