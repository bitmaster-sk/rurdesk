import { Injectable, computed, signal } from '@angular/core';
import { Role, ROLE_RANK } from '../../shared/constants/role.enum';

@Injectable({ providedIn: 'root' })
export class AclStore {
    private readonly role = signal<Role | null>(null);

    // canDeleteIssue, canDeleteProject and canDeleteMembers have no caller yet:
    // those affordances are shown to everyone and the server's 403 stops them.
    // Wiring them is a UX fix, not a security one.

    // member+
    public readonly canCreateIssue = computed(() => this.atLeast(Role.Member));
    public readonly canUpdateIssue = computed(() => this.atLeast(Role.Member));
    public readonly canDeleteIssue = computed(() => this.atLeast(Role.Member));

    // owner
    public readonly canUpdateProject = computed(() => this.atLeast(Role.Owner));
    public readonly canDeleteProject = computed(() => this.atLeast(Role.Owner));
    public readonly canManageSeverity = computed(() => this.atLeast(Role.Owner));
    public readonly canManageState = computed(() => this.atLeast(Role.Owner));
    public readonly canReadMembers = computed(() => this.atLeast(Role.Owner));
    public readonly canCreateMembers = computed(() => this.atLeast(Role.Owner));
    public readonly canUpdateMembers = computed(() => this.atLeast(Role.Owner));
    public readonly canDeleteMembers = computed(() => this.atLeast(Role.Owner));

    public readonly canReadGitIntegration = computed(() => this.atLeast(Role.Member));
    public readonly canManageGitIntegration = computed(() => this.atLeast(Role.Owner));

    public setRole(role: Role | null): void {
        this.role.set(role);
    }

    private atLeast(minimum: Role): boolean {
        const current = this.role();
        return current !== null && ROLE_RANK[current] >= ROLE_RANK[minimum];
    }
}
