import { AclStore } from './acl.store';
import { Role } from '../../shared/constants/role.enum';

describe('AclStore', () => {
    let store: AclStore;

    beforeEach(() => {
        store = new AclStore();
    });

    it('returns false for all permissions when role is null', () => {
        store.setRole(null);
        expect(store.canCreateIssue()).toBe(false);
        expect(store.canUpdateProject()).toBe(false);
        expect(store.canReadMembers()).toBe(false);
    });

    it('viewer can only read — no write permissions', () => {
        store.setRole(Role.Viewer);
        expect(store.canCreateIssue()).toBe(false);
        expect(store.canUpdateIssue()).toBe(false);
        expect(store.canDeleteIssue()).toBe(false);
        expect(store.canUpdateProject()).toBe(false);
        expect(store.canReadMembers()).toBe(false);
    });

    it('member can create/update/delete issues but not manage project', () => {
        store.setRole(Role.Member);
        expect(store.canCreateIssue()).toBe(true);
        expect(store.canUpdateIssue()).toBe(true);
        expect(store.canDeleteIssue()).toBe(true);
        expect(store.canUpdateProject()).toBe(false);
        expect(store.canReadMembers()).toBe(false);
    });

    it('owner has all permissions', () => {
        store.setRole(Role.Owner);
        expect(store.canCreateIssue()).toBe(true);
        expect(store.canUpdateProject()).toBe(true);
        expect(store.canDeleteProject()).toBe(true);
        expect(store.canManageSeverity()).toBe(true);
        expect(store.canManageState()).toBe(true);
        expect(store.canReadMembers()).toBe(true);
        expect(store.canCreateMembers()).toBe(true);
        expect(store.canUpdateMembers()).toBe(true);
        expect(store.canDeleteMembers()).toBe(true);
    });

    it('role change is reactive', () => {
        store.setRole(Role.Viewer);
        expect(store.canCreateIssue()).toBe(false);
        store.setRole(Role.Member);
        expect(store.canCreateIssue()).toBe(true);
    });
});
