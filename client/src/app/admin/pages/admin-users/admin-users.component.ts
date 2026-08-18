import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { AdminApi } from '../../api/admin.api.service';
import { AdminUser, UserCreatedEvent } from '../../model/admin-user.model';
import { ToastNotificationService } from 'src/app/core/toast-notification.service';
import { ApiError } from 'src/app/shared/model/api-error.model';

@Component({
    selector: 'app-admin-users',
    templateUrl: './admin-users.component.html',
    styleUrls: ['./admin-users.component.scss'],
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class AdminUsersComponent implements OnInit {
    private readonly adminApi = inject(AdminApi);
    private readonly sToast = inject(ToastNotificationService);

    protected readonly users = signal<AdminUser[]>([]);
    protected readonly draggedUser = signal<AdminUser | null>(null);
    protected readonly showCreate = signal(false);
    protected readonly editUser = signal<AdminUser | null>(null);
    protected readonly showEdit = signal(false);
    protected readonly keysBot = signal<AdminUser | null>(null);
    protected readonly showKeys = signal(false);
    // Seed the keys window with the tokens minted during one-shot creation so
    // they are revealed once; null when managing an existing bot.
    protected readonly keysPresetKey = signal<string | null>(null);
    protected readonly keysPresetGatewayToken = signal<string | null>(null);

    public ngOnInit(): void {
        this.loadUsers();
    }

    private loadUsers(): void {
        this.adminApi.listUsers$().subscribe(users => this.users.set(users));
    }

    protected onOpenCreate(): void {
        this.showCreate.set(true);
    }

    protected onUserCreated(event: UserCreatedEvent): void {
        this.loadUsers();
        // A bot lands straight in its keys window with the freshly-minted tokens
        // revealed once. Human users have no keys — nothing more to show.
        if (event.user.isBot) {
            this.keysBot.set(event.user);
            this.keysPresetKey.set(event.user.rawKey ?? null);
            this.keysPresetGatewayToken.set(event.gatewayToken);
            this.showKeys.set(true);
        }
    }

    protected onOpenEdit(user: AdminUser): void {
        this.editUser.set(user);
        this.showEdit.set(true);
    }

    protected onUserUpdated(): void {
        this.loadUsers();
    }

    protected onToggleAdmin(user: AdminUser): void {
        this.adminApi.setAdmin$(user.idUser, !user.isAdmin).subscribe({
            next: () => this.loadUsers(),
            error: (err: unknown) =>
                this.sToast.showError(ApiError.translateKeyOf(err) ?? 'ADMIN.ACTION_FAILED')
        });
    }

    protected onConfirmDelete(user: AdminUser): void {
        this.adminApi.deleteUser$(user.idUser).subscribe({
            next: () => this.loadUsers(),
            error: (err: unknown) =>
                this.sToast.showError(ApiError.translateKeyOf(err) ?? 'ADMIN.ACTION_FAILED')
        });
    }

    protected onOpenKeys(user: AdminUser): void {
        this.keysBot.set(user);
        this.keysPresetKey.set(null);
        this.keysPresetGatewayToken.set(null);
        this.showKeys.set(true);
    }

    protected onUserDragStart(event: DragEvent, user: AdminUser): void {
        this.draggedUser.set(user);
        // Firefox refuses to start a native drag unless dataTransfer is written.
        // The drop side reads the payload from the draggedUser signal, not from
        // dataTransfer; idUser here is a deliberate contract for future cross-window use.
        event.dataTransfer?.setData('text/plain', String(user.idUser));
        if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copy';
    }

    protected onUserDragEnd(): void {
        this.draggedUser.set(null);
    }
}
