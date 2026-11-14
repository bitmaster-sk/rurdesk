import { AppSettings, UpdateAppSettingsReq } from 'src/app/core/settings/model/app-settings.model';

export interface SettingsSection {
    patch(settings: AppSettings): void;
    toRequest(): UpdateAppSettingsReq;
}
