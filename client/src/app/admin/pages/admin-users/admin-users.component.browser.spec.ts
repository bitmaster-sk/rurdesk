import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { AdminApi } from '../../api/admin.api.service';
import { AdminUser } from '../../model/admin-user.model';
import { AdminUsersComponent } from './admin-users.component';

/**
 * Regression for the pDraggable → native HTML5 drag migration. The template is
 * trimmed to just the drag handle so we exercise the REAL event binding
 * (`draggable="true"`, `(dragstart)`, `(dragend)`) against the real component —
 * not a direct method call. Firefox needs `dataTransfer` written in dragstart
 * or the drag never starts, so the handler must populate it.
 */
describe('AdminUsersComponent — native drag source (browser)', () => {
    const user = { idUser: 7, name: 'Ada', email: 'ada@x.io' } as AdminUser;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [AdminUsersComponent],
            providers: [
                { provide: AdminApi, useValue: { listUsers$: () => of([user]) } },
                { provide: ToastNotificationService, useValue: {} }
            ]
        })
            .overrideComponent(AdminUsersComponent, {
                set: {
                    template: `@for (u of users(); track u.idUser) {
                        <span class="drag-handle" draggable="true"
                              (dragstart)="onUserDragStart($event, u)"
                              (dragend)="onUserDragEnd()"></span>
                    }`
                }
            })
            .compileComponents();
    });

    function render() {
        const fixture = TestBed.createComponent(AdminUsersComponent);
        fixture.detectChanges();
        const comp = fixture.componentInstance as unknown as {
            draggedUser: () => AdminUser | null;
        };
        const handle = fixture.nativeElement.querySelector('.drag-handle') as HTMLElement;
        return { fixture, comp, handle };
    }

    it('dragstart sets draggedUser and writes the payload to dataTransfer', () => {
        const { comp, handle } = render();
        const dataTransfer = new DataTransfer();
        handle.dispatchEvent(new DragEvent('dragstart', { dataTransfer, bubbles: true }));

        expect(comp.draggedUser()).toBe(user);
        expect(dataTransfer.getData('text/plain')).toBe('7');
    });

    it('dragend clears draggedUser', () => {
        const { comp, handle } = render();
        handle.dispatchEvent(
            new DragEvent('dragstart', { dataTransfer: new DataTransfer(), bubbles: true })
        );
        expect(comp.draggedUser()).toBe(user);

        handle.dispatchEvent(new DragEvent('dragend', { bubbles: true }));
        expect(comp.draggedUser()).toBeNull();
    });
});
