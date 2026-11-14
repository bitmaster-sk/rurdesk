import { AdminSettingsSection } from '../entity/admin-settings-section.entity';

export const ADMIN_SETTINGS_SECTIONS: readonly AdminSettingsSection[] = [
    { path: 'general', icon: 'settings', label: 'ADMIN.SETTINGS_NAV.GENERAL' },
    { path: 'agents', icon: 'sparkles', label: 'ADMIN.SETTINGS_NAV.AGENTS' },
    { path: 'sso', icon: 'shield-lock', label: 'ADMIN.SETTINGS_NAV.SSO' }
];
